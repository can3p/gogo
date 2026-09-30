package config

import (
	"errors"

	"github.com/can3p/gogo/settings"
)

// Config is the Mailjet sender's settings. Embed it as a go-flags group with
// a namespace, `group:"Mailjet" namespace:"mj" env-namespace:"MJ"`, to read
// MJ_APIKEY_PUBLIC, MJ_APIKEY_PRIVATE and MJ_API_BASE.
type Config struct {
	ApiKeyPublic  string          `long:"api-key-public" env:"APIKEY_PUBLIC" description:"Mailjet API key public" required:"true"`
	ApiKeyPrivate settings.Secret `long:"api-key-private" env:"APIKEY_PRIVATE" description:"Mailjet API key private" required:"true"`
	// BaseURL is the root of the Mailjet API. Point it at a Mailjet mock, such
	// as tommy (github.com/can3p/tommy), in development and tests. Empty means
	// Mailjet's own.
	BaseURL string `long:"api-base" env:"API_BASE" description:"Mailjet API root, such as a mock's http://localhost:8822; Mailjet's own if empty"`
}

func (c *Config) Validate() error {
	if c.ApiKeyPublic == "" || c.ApiKeyPrivate == "" {
		return errors.New("mailjet API key public and private are required")
	}

	return nil
}
