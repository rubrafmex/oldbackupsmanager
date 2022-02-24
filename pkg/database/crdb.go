package database

import (
	"database/sql"
	"net/url"
	"strings"
	"time"

	_ "github.com/lib/pq" // postgres driver
)

func NewCrdb(cfg Config) (*sql.DB, error) {
	const driver = "postgres" // depends on the driver, currently lib/pq
	dsn := &url.URL{
		Scheme:   driver,
		Host:     cfg.Host,
		Path:     cfg.Database,
		RawQuery: strings.Join(cfg.Options, "&"),
	}

	if cfg.Password != "" {
		dsn.User = url.UserPassword(cfg.User, cfg.Password)
	} else {
		dsn.User = url.User(cfg.User)
	}

	db, err := sql.Open(driver, dsn.String())
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	// If we do not set a limit, queries will return a "too many clients" or "remote host refused" error when we
	// pass the limit. Now we do set a limit, Go will wait for an idle connection and execute the query normally.
	// Do make sure to use a context with a timeout in queries to prevent waiting too long.
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
		db.SetMaxIdleConns(cfg.MaxOpenConns)
	}

	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(3 * time.Minute)

	return db, nil
}
