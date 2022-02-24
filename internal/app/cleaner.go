package app

import (
	"context"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"io/ioutil"
	"os"
	"path"
	"time"
)

type Cleaner struct {
	ctx               context.Context
	logger            *zap.Logger
	sem               *semaphore.Weighted
	fileSystemWrapper *FileSystemWrapper
}

func NewCleaner(ctx context.Context, logger *zap.Logger, sem *semaphore.Weighted, fileSystemWrapper *FileSystemWrapper) *Cleaner {
	return &Cleaner{
		ctx:               ctx,
		logger:            logger,
		sem:               sem,
		fileSystemWrapper: fileSystemWrapper,
	}
}

func (c *Cleaner) SanityClean(durationInMinutes int) {
	ticker := time.NewTicker(time.Duration(durationInMinutes) * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-c.ctx.Done():
				return
			case t := <-ticker.C:
				c.cleanTempDirsContent(t)
			}
		}
	}()
}

func (c *Cleaner) cleanTempDirsContent(t time.Time) {
	// get and release local semaphore
	semErr := c.sem.Acquire(c.ctx, 1)
	defer func() {
		if semErr == nil {
			c.sem.Release(1)
		}
	}()
	if semErr != nil {
		c.logger.Error("cleanTempDirsContent: unable to obtain local semaphore")
		return
	}

	c.logger.Info("cleanTempDirsContent: cleaning temp dirs...", zap.Time("at", t))
	if err := c.removeDirContents(c.fileSystemWrapper.PathZips()); err != nil {
		c.logger.Warn("cleanTempDirsContent: error while cleaning temp dir", zap.String("directory", c.fileSystemWrapper.PathZips()))
	}
	if err := c.removeDirContents(c.fileSystemWrapper.PathEncrypted()); err != nil {
		c.logger.Warn("cleanTempDirsContent: error while cleaning temp dir", zap.String("directory", c.fileSystemWrapper.PathEncrypted()))
	}
	if err := c.removeDirContents(c.fileSystemWrapper.PathDecrypted()); err != nil {
		c.logger.Warn("cleanTempDirsContent: error while cleaning temp dir", zap.String("directory", c.fileSystemWrapper.PathDecrypted()))
	}
	if err := c.removeDirContents(c.fileSystemWrapper.PathGSDownloads()); err != nil {
		c.logger.Warn("cleanTempDirsContent: error while cleaning temp dir", zap.String("directory", c.fileSystemWrapper.PathGSDownloads()))
	}
}

func (c *Cleaner) removeDirContents(toBeRemoved string) error {
	dir, err := ioutil.ReadDir(toBeRemoved)
	if err != nil {
		return err
	}
	for _, d := range dir {
		err := os.RemoveAll(path.Join([]string{toBeRemoved, d.Name()}...))
		if err != nil {
			return err
		}
	}
	c.logger.Info("removeDirContents: directory contents successfully deleted", zap.String("directory", toBeRemoved))
	return nil
}
