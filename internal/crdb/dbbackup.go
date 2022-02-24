package crdb

import (
	"database/sql"
	"fmt"
	"gitlab.cmpayments.local/payments-gateway/backupsmanager/pkg/database"
	"go.uber.org/zap"
	"net/url"
	"path"
)

type Wrapper struct {
	logger             *zap.Logger
	db                 *sql.DB
	fileServerEndpoint string
}

func NewWrapper(logger *zap.Logger, db *sql.DB, fileServerEndpoint string) *Wrapper {
	return &Wrapper{
		logger:             logger,
		db:                 db,
		fileServerEndpoint: fileServerEndpoint,
	}
}

func (w *Wrapper) TriggerBackup(backupsDir string) error {
	u, _ := url.Parse(w.fileServerEndpoint)
	u.Path = path.Join(u.Path, backupsDir)
	query := fmt.Sprintf("BACKUP INTO '%s' AS OF SYSTEM TIME '-10s'", u.String())

	if _, err := w.db.Exec(query); err != nil {
		w.logger.Error("TriggerBackup: error triggering backup", zap.Error(err))
		return &database.Error{Err: err}
	}
	return nil
}
