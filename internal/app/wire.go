package app

// Container stores the assembled application dependencies.
type Container struct {
	App *Application
}

// Wire builds the application container.
func Wire(cfg BootstrapConfig) (*Container, error) {
	application, err := Bootstrap(cfg)
	if err != nil {
		return nil, err
	}
	return &Container{App: application}, nil
}
