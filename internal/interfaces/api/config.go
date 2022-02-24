package api

import (
	"errors"
	"fmt"
	"net/url"
)

type Config struct {
	// The public URL where this application is being served. Must not end in a slash.
	// A request to this URL must hit our http.Server listening on Listen
	BaseURL string
	// Addr for the HTTP server to listen on for inbound requests
	Listen string
}

func (c Config) Assert() error {
	if l := len(c.BaseURL); l > 0 && c.BaseURL[l-1] == '/' {
		return errors.New("BaseURL must not end in slash")
	}
	if baseURL, err := url.Parse(c.BaseURL); err != nil {
		return fmt.Errorf("BaseURL is invalid: %w", err)
	} else if !baseURL.IsAbs() {
		return errors.New("BaseURL must be absolute")
	} else if baseURL.Opaque != "" {
		return errors.New("BaseURL must not be opaque")
	}
	if c.Listen == "" {
		return errors.New("c.Listen can't be empty")
	}
	return nil
}
