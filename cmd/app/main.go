package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"gitlab.cmpayments.local/libraries-go/configuration"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/app"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/crdb"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/gcp"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/interfaces/api"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/pkg/ctxt"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/pkg/database"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/pkg/server"
	webdav2 "gitlab.cmpayments.local/payments-gateway/backupsmanager/pkg/webdav"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"log"
	"net/http"
	"os"
)

type Config struct {
	// WorkingDir working dir path
	WorkingDir string
	// SanityCleanIntervalInMinutes interval to clean local directories
	SanityCleanIntervalInMinutes int
	// API Config
	API api.Config
	// DB is the database Config
	DB database.Config
	// GCP is the Google cloud storage Config
	GCP gcp.Config
}

func (c Config) Assert() error {
	if len(c.WorkingDir) == 0 {
		return errors.New("c.WorkingDir is not defined")
	}
	if c.SanityCleanIntervalInMinutes < 10 {
		return errors.New("c.SanityCleanIntervalInMinutes should be greater than 10")
	}
	if err := c.API.Assert(); err != nil {
		return fmt.Errorf("%w in API Config", err)
	}
	if err := c.DB.Assert(); err != nil {
		return fmt.Errorf("%w in DB Config", err)
	}
	return nil
}

func main() {
	// Config
	configFile := flag.String(`Config`, `Config.ini`, `Configuration file`)
	flag.Parse()
	var cfg Config
	if err := configuration.LoadIni(*configFile, &cfg); err != nil {
		panic(err)
	} else if err := cfg.Assert(); err != nil {
		panic(fmt.Sprintf("configuration error: %v", err))
	}

	// Context
	ctx := ctxt.WithSignalTrap(context.Background(), os.Kill, os.Interrupt)

	// Logger
	logger, _ := zap.NewProduction()
	defer func() {
		if err := logger.Sync(); err != nil {
			log.Println("error in logger sync", err.Error())
		}
	}()

	// Database
	logger.Debug("main: connecting to database", zap.String("host", cfg.DB.Host), zap.String("database", cfg.DB.Database))
	db, err := database.NewCrdb(cfg.DB)
	if err != nil {
		panic(fmt.Errorf("error connecting to database %s: %w", cfg.DB.Host, err))
	}
	defer db.Close()

	// Semaphore
	// this service has backups, fromBucket and SanityClean processes which are using the same file system resources.
	// sem will avoid collision issues between these internal processes.
	sem := semaphore.NewWeighted(int64(10))

	// Components
	fileSystemWrapper := app.NewFileSystemWrapper(ctx, logger, cfg.WorkingDir)
	crdbWrapper := crdb.NewWrapper(logger, db, fileServerEndpoint(cfg.API.BaseURL))
	webdavWrapper := webdav2.NewWrapper(logger, fileSystemWrapper.PathBackups())
	zipper := app.NewZipper(ctx, logger, sem, fileSystemWrapper)
	encryptor := app.NewEncryptor(ctx, logger, sem, fileSystemWrapper.PathEncrypted(), fileSystemWrapper.PathDecrypted())
	gcsIntegrator := gcp.NewGCSIntegrator(ctx, logger, sem, fileSystemWrapper.PathGSDownloads(), cfg.GCP)
	cleaner := app.NewCleaner(ctx, logger, sem, fileSystemWrapper)

	mux := http.NewServeMux()

	mux.Handle("/probes/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// setup handlers
	api.RegisterHandler(ctx, logger, cfg.GCP.Enabled, sem, crdbWrapper, webdavWrapper, zipper, encryptor, gcsIntegrator, fileSystemWrapper, mux)

	// set up cleanup routine
	cleaner.SanityClean(cfg.SanityCleanIntervalInMinutes)

	serverAddr := cfg.API.Listen
	srv := server.New(mux, serverAddr)

	// start server
	logger.Info("main: server starting", zap.String("Addr", serverAddr))
	errSv := srv.ListenAndServe()
	if errSv != nil {
		logger.Fatal("main: server failed to start: %v", zap.Error(errSv))
	}
}

func fileServerEndpoint(apiBaseUrl string) string {
	return apiBaseUrl + api.Paths.Backups()
}
