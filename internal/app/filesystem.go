package app

import (
	"context"
	"go.uber.org/zap"
)

type FileSystemWrapper struct {
	ctx        context.Context
	logger     *zap.Logger
	workingDir string
}

func NewFileSystemWrapper(ctx context.Context, logger *zap.Logger, workingDir string) *FileSystemWrapper {
	return &FileSystemWrapper{
		ctx:        ctx,
		logger:     logger,
		workingDir: workingDir,
	}
}

func (z *FileSystemWrapper) PathBackups() string {
	return z.workingDir + "/backups"
}

func (z *FileSystemWrapper) PathZips() string {
	return z.workingDir + "/zips"
}

func (z *FileSystemWrapper) PathEncrypted() string {
	return z.workingDir + "/encrypted"
}

func (z *FileSystemWrapper) PathDecrypted() string {
	return z.workingDir + "/decrypted"
}

func (z *FileSystemWrapper) PathGSDownloads() string {
	return z.workingDir + "/gsdownloads"
}
