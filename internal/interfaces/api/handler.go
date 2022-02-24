package api

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/app"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/crdb"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/gcp"
	webdav2 "gitlab.cmpayments.local/payments-gateway/backupsmanager/pkg/webdav"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Handler struct {
	ctx               context.Context
	logger            *zap.Logger
	sem               *semaphore.Weighted
	gcpIntegration    bool
	crdbWrapper       *crdb.Wrapper
	webdavWrapper     *webdav2.Wrapper
	zipper            *app.Zipper
	encryptor         *app.Encryptor
	gcsIntegrator     *gcp.GCSIntegrator
	fileSystemWrapper *app.FileSystemWrapper
}

func RegisterHandler(ctx context.Context, logger *zap.Logger, gcpIntegration bool, sem *semaphore.Weighted, crdbWrapper *crdb.Wrapper, webdavWrapper *webdav2.Wrapper, zipper *app.Zipper, encryptor *app.Encryptor, gcsIntegrator *gcp.GCSIntegrator, fileSystemWrapper *app.FileSystemWrapper, mux *http.ServeMux) {
	handler := &Handler{
		ctx:               ctx,
		logger:            logger,
		sem:               sem,
		gcpIntegration:    gcpIntegration,
		crdbWrapper:       crdbWrapper,
		webdavWrapper:     webdavWrapper,
		zipper:            zipper,
		encryptor:         encryptor,
		gcsIntegrator:     gcsIntegrator,
		fileSystemWrapper: fileSystemWrapper,
	}

	mux.Handle(endpointCRDBBackup, http.StripPrefix("/crdbBackup", handler.pathValidationInterceptor(http.HandlerFunc(handler.TriggerCRDBBackup))))

	mux.Handle(endpointBackups, http.StripPrefix("/backups", handler.backupsDirectoriesInterceptor(handler.webdavWrapper.Handler)))

	mux.Handle(endpointFromBucket, http.StripPrefix("/fromBucket", handler.pathValidationInterceptor(http.HandlerFunc(handler.fromBucket))))

	mux.Handle(endpointListBackups, http.HandlerFunc(handler.listBackups))
}

func (h *Handler) pathValidationInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Base(r.URL.Path) == "/" || strings.Count(r.URL.Path, "/") != 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			resp := make(map[string]string)
			resp["message"] = "Invalid request"
			jsonResp, _ := json.Marshal(resp)
			_, _ = w.Write(jsonResp)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) TriggerCRDBBackup(w http.ResponseWriter, r *http.Request) {
	err := h.crdbWrapper.TriggerBackup(r.URL.Path)
	if err != nil {
		internalServerErrResponse(w, "Some Error Occurred (while triggering TriggerCRDBBackup")
		return
	}

	if h.gcpIntegration {
		latest, err := ioutil.ReadFile(path.Join(h.fileSystemWrapper.PathBackups(), r.URL.Path, "LATEST"))
		if err != nil {
			h.logger.Error("TriggerCRDBBackup: error finding LATEST file", zap.Error(err))
			internalServerErrResponse(w, "Some Error Occurred (while reading LATEST file from backups")
			return
		}

		latestBackupDir := path.Join(h.fileSystemWrapper.PathBackups(), r.URL.Path, string(latest))
		if _, err := os.Stat(latestBackupDir); err != nil {
			h.logger.Error("TriggerCRDBBackup: LATEST backup dir not found", zap.Error(err))
			internalServerErrResponse(w, "Some Error Occurred (while getting LATEST backup")
			return
		}

		zipperResultStream := h.zipper.Zip(latestBackupDir)
		encryptorResultStream := h.encryptor.Encrypt(zipperResultStream)
		h.gcsIntegrator.UploadToStorage(encryptorResultStream)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["message"] = "Backup triggered successfully"
	jsonResp, _ := json.Marshal(resp)
	_, _ = w.Write(jsonResp)
}

// backupsDirectoriesInterceptor intercepts HTTP requests and creates local directories needed
func (h *Handler) backupsDirectoriesInterceptor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get and release local semaphore
		semErr := h.sem.Acquire(h.ctx, 1)
		defer func() {
			if semErr == nil {
				h.sem.Release(1)
			}
		}()
		if semErr != nil {
			h.logger.Error("backupsDirectoriesInterceptor: unable to obtain local semaphore")
			internalServerErrResponse(w, "Some Error Occurred")
			return
		}

		switch r.Method {
		case "PUT":
			// create subdirectories if needed
			if _, err := os.Stat(path.Dir(r.URL.Path)); os.IsNotExist(err) {
				if err := os.MkdirAll(path.Join(h.fileSystemWrapper.PathBackups(), path.Dir(r.URL.Path)), 0700); err != nil {
					internalServerErrResponse(w, "Some Error Occurred")
					return
				}
			}
		}

		// continue with next handler
		next.ServeHTTP(w, r)
	})
}

func internalServerErrResponse(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["message"] = message
	jsonResp, _ := json.Marshal(resp)
	_, _ = w.Write(jsonResp)
}

func (h *Handler) fromBucket(w http.ResponseWriter, r *http.Request) {
	// get and release local semaphore
	semErr := h.sem.Acquire(h.ctx, 1)
	defer func() {
		if semErr == nil {
			h.sem.Release(1)
		}
	}()
	if semErr != nil {
		h.logger.Error("fromBucket: unable to obtain local semaphore")
		internalServerErrResponse(w, "Some Error Occurred")
		return
	}

	fileName := path.Base(r.URL.Path)
	downloadedFile, err := h.gcsIntegrator.DownloadFromBucket(fileName)
	if err != nil {
		internalServerErrResponse(w, "Some Error Occurred (while downloading)")
		return
	}

	zipFile, err := h.encryptor.DecryptFileAs(downloadedFile, ".zip")
	if err != nil {
		internalServerErrResponse(w, "Some Error Occurred (while decrypting)")
		return
	}

	backupResult, err := h.useZipAsBackup(zipFile)
	if err != nil {
		internalServerErrResponse(w, "Some Error Occurred")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["message"] = fmt.Sprintf("Backup result: %s", backupResult)
	jsonResp, _ := json.Marshal(resp)
	_, _ = w.Write(jsonResp)
}

func (h *Handler) useZipAsBackup(zipFile string) (string, error) {
	// check if directory already exists under backups
	fileName := path.Base(zipFile)
	backupsDirFileBased := "/" + strings.Replace(strings.Replace(fileName, ".zip", "", -1), "_", "/", -1)
	backupsDirFullPath := path.Join(h.fileSystemWrapper.PathBackups(), backupsDirFileBased)
	if _, err := os.Stat(backupsDirFullPath); err == nil {
		h.logger.Warn("useZipAsBackup: skipping, file already exists", zap.String("backupsDirFullPath", backupsDirFullPath))
		return fmt.Sprintf("downloaded data from bucket ignored, '%s' already exists as backup directory", backupsDirFullPath), nil
	}

	// directory does not exist, continue with unzip process
	backupsDestiny := path.Dir(backupsDirFullPath)
	err := h.zipper.UnzipSource(zipFile, backupsDestiny)
	if err != nil {
		h.logger.Error("useZipAsBackup: error while unzipping file as backup", zap.String("zipFile", zipFile), zap.String("backupsDestiny", backupsDestiny))
		return "", fmt.Errorf("error while using as backup: %v", err)
	}

	_, backups := path.Split(h.fileSystemWrapper.PathBackups())
	result := fmt.Sprintf("cockroach backup ready to use with command: 'RESTORE TABLE [DATABASE_NAME].[TABLE_NAME] FROM 'http://[THIS_SERVICE_IP]:[THIS_SERVICE_PORT]/%s'", path.Join(backups, backupsDirFileBased))
	return result, nil
}

func (h *Handler) listBackups(w http.ResponseWriter, r *http.Request) {
	var backupDataPaths []string
	err := filepath.Walk(h.fileSystemWrapper.PathBackups(),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				filesInDir, err := ioutil.ReadDir(path)
				if err != nil {
					return err
				}
				for _, f := range filesInDir {
					if f.IsDir() && f.Name() == "data" { // only consider backup directories that contain data
						backupDataPaths = append(backupDataPaths, path)
						return nil
					}
				}
			}
			return nil
		})
	if err != nil {
		internalServerErrResponse(w, "Some Error Occurred")
		return
	}

	// create response
	backupsResponse := make(map[string]*[]string)
	for _, fullBackupsDataPath := range backupDataPaths {
		backupsPath := strings.Replace(fullBackupsDataPath, h.fileSystemWrapper.PathBackups(), "", -1)
		// get key
		pathElements := strings.Split(backupsPath, "/")
		backupRootDir := pathElements[1]
		// get content
		backupRootPathFiltered := strings.Join(strings.SplitN(backupsPath, "/", 4), "/")
		if strings.Count(backupRootPathFiltered, "/") == 4 {
			// create or update response map entry
			if contents, exists := backupsResponse[backupRootDir]; exists {
				*contents = append(*contents, backupRootPathFiltered)
			} else {
				backupsResponse[backupRootDir] = &[]string{backupRootPathFiltered}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	jsonResp, _ := json.Marshal(backupsResponse)
	_, _ = w.Write(jsonResp)
}
