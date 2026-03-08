package app

import legacyapp "github.com/vigo999/ms-cli/app"

// BootstrapConfig is the startup configuration for the application wiring.
type BootstrapConfig = legacyapp.BootstrapConfig

// Application is the composed runtime application.
type Application = legacyapp.Application

// Bootstrap wires top-level dependencies.
func Bootstrap(cfg BootstrapConfig) (*Application, error) {
	return legacyapp.Bootstrap(cfg)
}
