package app

import (
	"errors"
	"os"
	"strings"
)

type factoryServerConfig struct {
	ServerURL string
	Token     string
	Source    string
}

func resolveFactoryServerConfig() factoryServerConfig {
	if config := factoryServerConfigFromEnv(); config.Configured() {
		return config
	}
	cred, err := loadCredentials()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return factoryServerConfig{}
		}
		return factoryServerConfig{}
	}
	config := factoryServerConfig{
		ServerURL: strings.TrimSpace(cred.ServerURL),
		Token:     strings.TrimSpace(cred.Token),
		Source:    "credentials.json",
	}
	if config.Configured() {
		return config
	}
	return factoryServerConfig{}
}

func factoryServerConfigFromEnv() factoryServerConfig {
	config := factoryServerConfig{
		ServerURL: strings.TrimSpace(os.Getenv("MSCLI_FACTORY_SERVER_URL")),
		Token:     strings.TrimSpace(os.Getenv("MSCLI_FACTORY_TOKEN")),
	}
	if config.Configured() {
		config.Source = "env"
	}
	return config
}

func (c factoryServerConfig) Configured() bool {
	return c.ServerURL != "" && c.Token != ""
}
