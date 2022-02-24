package database

import (
	"errors"
)

type Config struct {
	Host         string
	User         string
	Password     string
	Database     string
	Options      []string
	MaxOpenConns int
}

func (c Config) Assert() error {
	if c.Host == "" {
		return errors.New("empty value is not allowed for convar Host")
	}
	if c.User == "" {
		return errors.New("empty value is not allowed for convar User")
	}
	if c.Database == "" {
		return errors.New("empty value is not allowed for convar Database")
	}
	if c.MaxOpenConns < 0 {
		return errors.New("minimum value for convar MaxOpenConns is 0")
	}

	return nil
}
