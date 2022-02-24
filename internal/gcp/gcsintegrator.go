package gcp

import (
	"cloud.google.com/go/storage"
	"context"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/internal/app"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/option"
	"io/ioutil"
	"os"
	"path"
)

type GCSIntegrator struct {
	ctx               context.Context
	logger            *zap.Logger
	sem               *semaphore.Weighted
	downloadsRootPath string
	client            *storage.Client
	bucket            *storage.BucketHandle
	bucketName        string
}

func NewGCSIntegrator(ctx context.Context, logger *zap.Logger, sem *semaphore.Weighted, downloadsRootPath string, config Config) *GCSIntegrator {
	if err := os.MkdirAll(downloadsRootPath, os.ModePerm); err != nil && !os.IsExist(err) {
		logger.Fatal("FATAL %v", zap.Error(err))
	}

	sDec, _ := b64.StdEncoding.DecodeString(config.Base64EncodedJsonKey)
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(sDec))
	if err != nil {
		logger.Warn("NewGCSIntegrator: failed to create gs storage  client", zap.Error(err))
	}

	return &GCSIntegrator{
		ctx:               ctx,
		logger:            logger,
		sem:               sem,
		downloadsRootPath: downloadsRootPath,
		client:            client,
		bucket:            client.Bucket(config.GCSBucketName),
		bucketName:        config.GCSBucketName,
	}
}

func (g *GCSIntegrator) UploadToStorage(toBucket <-chan app.DTO) {
	go func() {
		select {
		case <-g.ctx.Done():
			return
		case tb, more := <-toBucket:
			if more {
				g.uploadToBucket(tb)
			}
			return
		}
	}()
}

func (g *GCSIntegrator) uploadToBucket(toBucket app.DTO) app.DTO {
	if g.client == nil {
		g.logger.Warn("uploadToBucket: skipping, storage client is not set")
		return app.NewDTOInstance(fmt.Errorf("skipping upload to bucket, storage client is not set"), "")
	}

	if toBucket.Err() != nil {
		g.logger.Warn("uploadToBucket: skipping, source has already an error", zap.Error(toBucket.Err()))
		return app.NewDTOInstance(fmt.Errorf("skipping upload to bucket, source has already an error"), "")
	}

	// get and release local semaphore
	semErr := g.sem.Acquire(g.ctx, 1)
	defer func() {
		if semErr == nil {
			g.sem.Release(1)
		}
	}()
	if semErr != nil {
		g.logger.Error("uploadToBucket: skipping, unable to obtain local semaphore")
		return app.NewDTOInstance(fmt.Errorf("error while encrypting: %v", errors.New("skipping, unable to obtain local semaphore")), "")
	}

	// start uploading
	fileName := path.Base(toBucket.Content())

	wc := g.bucket.Object(fileName).NewWriter(g.ctx)
	wc.ContentType = "text/plain"

	ciphered, err := ioutil.ReadFile(toBucket.Content())
	if err != nil {
		g.logger.Error("uploadToBucket: unable read data to be uploaded in bucket", zap.Error(err))
		return app.NewDTOInstance(fmt.Errorf("error while uploading to bucket: %v", err), "")
	}

	if _, err := wc.Write(ciphered); err != nil {
		g.logger.Error("uploadToBucket: unable to write data to bucket", zap.Error(err))
		return app.NewDTOInstance(fmt.Errorf("error while uploading to bucket: %v", err), "")
	}
	if err := wc.Close(); err != nil {
		g.logger.Error("uploadToBucket: unable to close bucket", zap.Error(err))
		return app.NewDTOInstance(fmt.Errorf("error while uploading to bucket: %v", err), "")
	}

	return app.NewDTOInstance(nil, path.Join(g.bucketName, toBucket.Content()))
}

func (g *GCSIntegrator) DownloadFromBucket(fileName string) (string, error) {
	if g.client == nil {
		g.logger.Error("downloadFromBucket:skipping, storage client is not set")
		return "", errors.New("skipping download from bucket, storage client is not set")
	}

	rc, err := g.bucket.Object(fileName).NewReader(g.ctx)
	if err != nil {
		g.logger.Error("downloadFromBucket: unable to open file from bucket", zap.String("bucketName", g.bucketName), zap.String("fileName", fileName), zap.Error(err))
		return "", fmt.Errorf("error while downloading from bucket %v", err)
	}
	defer rc.Close()
	content, err := ioutil.ReadAll(rc)
	if err != nil {
		g.logger.Error("downloadFromBucket: unable to read data from bucket", zap.String("bucketName", g.bucketName), zap.String("fileName", fileName), zap.Error(err))
		return "", fmt.Errorf("error while downloading from bucket %v", err)
	}

	// Save back to file
	filePath := path.Join(g.downloadsRootPath, fileName)
	err = ioutil.WriteFile(filePath, content, 0777)
	if err != nil {
		g.logger.Error("downloadFromBucket: unable to write data from bucket into local directory", zap.Error(err))
		return "", fmt.Errorf("error while downloading from bucket %v", err)
	}
	return filePath, nil
}
