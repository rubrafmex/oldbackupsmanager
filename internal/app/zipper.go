package app

import (
	"archive/zip"
	"context"
	"fmt"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Zipper struct {
	ctx               context.Context
	logger            *zap.Logger
	sem               *semaphore.Weighted
	fileSystemWrapper *FileSystemWrapper
}

func NewZipper(ctx context.Context, logger *zap.Logger, sem *semaphore.Weighted, fileSystemWrapper *FileSystemWrapper) *Zipper {
	if err := os.MkdirAll(fileSystemWrapper.PathZips(), os.ModePerm); err != nil && !os.IsExist(err) {
		logger.Fatal("FATAL %v", zap.Error(err))
	}

	return &Zipper{
		ctx:               ctx,
		logger:            logger,
		sem:               sem,
		fileSystemWrapper: fileSystemWrapper,
	}
}

func (z *Zipper) Zip(backupDirPath string) <-chan DTO {
	resultStream := make(chan DTO)
	go func() {
		defer close(resultStream)
		backupsSubPath := strings.Replace(backupDirPath, z.fileSystemWrapper.PathBackups(), "", -1)
		fileName := strings.Replace(backupsSubPath[1:], "/", "_", -1) + ".zip"
		zipFilePath := path.Join(z.fileSystemWrapper.PathZips(), fileName)

		// get and release local semaphore
		semErr := z.sem.Acquire(z.ctx, 1)
		defer func() {
			if semErr == nil {
				z.sem.Release(1)
			}
		}()
		if semErr != nil {
			z.logger.Error("zip: skipping, unable to obtain local semaphore")
			return
		}

		if err := z.zipSource(backupDirPath, zipFilePath); err != nil {
			z.logger.Error("zip: error while creating zip file", zap.String("source", backupDirPath), zap.String("target", zipFilePath), zap.Error(err))
			return
		}
		resultStream <- NewDTOInstance(nil, zipFilePath)
	}()
	return resultStream
}

func (z *Zipper) zipSource(source, target string) error {
	// 1. Create a ZIP file and zip.Writer
	f, crerr := os.Create(target)
	defer func() {
		if crerr == nil {
			if cserr := f.Close(); cserr != nil {
				z.logger.Error("zipSource: error closing target file", zap.String("target", target), zap.Error(cserr))
			}
		}
	}()
	if crerr != nil {
		return crerr
	}

	writer := zip.NewWriter(f)
	defer writer.Close()

	// Go through all the files of the source
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create a local file header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// set compression
		header.Method = zip.Deflate

		// Set relative path of a file as the header name
		header.Name, err = filepath.Rel(filepath.Dir(source), path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			header.Name += "/"
		}

		// Create writer for the file header and save content of the file
		headerWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(headerWriter, f)
		return err
	})
}

func (z *Zipper) UnzipSource(source, destination string) error {
	// 1. Open the zip file
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	// 2. Get the absolute destination path
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}

	// 3. Iterate over zip files inside the archive and unzip each of them
	for _, f := range reader.File {
		err := unzipFile(f, destination)
		if err != nil {
			return err
		}
	}

	return nil
}

func unzipFile(f *zip.File, destination string) error {
	// 4. Check if file paths are not vulnerable to Zip Slip
	filePath := filepath.Join(destination, f.Name)
	if !strings.HasPrefix(filePath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	// 5. Create directory tree
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	// 6. Create a destination file for unzipped content
	destinationFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// 7. Unzip the content of a file and copy it to the destination file
	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer zippedFile.Close()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		return err
	}
	return nil
}
