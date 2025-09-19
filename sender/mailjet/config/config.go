package config

import "errors"

type Config struct {
	ApiKeyPublic  string `long:"api-key-public" env:"APIKEY_PUBLIC" description:"Mailjet API key public"`
	ApiKeyPrivate string `long:"api-key-private" env:"APIKEY_PRIVATE" description:"Mailjet API key private"`
}

func (c *Config) Validate() error {
	if c.ApiKeyPublic == "" || c.ApiKeyPrivate == "" {
		return errors.New("Mailjet API key public and private are required")
	}

	return nil
}
