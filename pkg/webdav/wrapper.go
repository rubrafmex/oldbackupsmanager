package webdav

import (
	"go.uber.org/zap"
	"golang.org/x/net/webdav"
	"log"
	"net/http"
	"os"
)

type Wrapper struct {
	logger  *zap.Logger
	Handler *webdav.Handler
}

func NewWrapper(logger *zap.Logger, localFileSystem string) *Wrapper {
	handler := &webdav.Handler{
		FileSystem: filesystem(logger, localFileSystem),
		LockSystem: webdav.NewMemLS(),
		Logger:     httpLogger(logger),
	}

	return &Wrapper{
		logger:  logger,
		Handler: handler,
	}
}

func filesystem(logger *zap.Logger, localFileSystem string) webdav.FileSystem {
	if err := os.MkdirAll(localFileSystem, os.ModePerm); err != nil && !os.IsExist(err) {
		log.Fatalf("FATAL %v", err)
	}
	logger.Info("filesystem: WEBDAV local filesystem", zap.String("local filesystem", localFileSystem))
	return webdav.Dir(localFileSystem)
}

func httpLogger(logger *zap.Logger) func(*http.Request, error) {
	return func(r *http.Request, err error) {
		if err != nil {
			logger.Error("httpLogger: WEBDAV REQUEST ERROR", zap.String("Method", r.Method), zap.String("URL", r.URL.String()), zap.Error(err))
		}
	}
}
