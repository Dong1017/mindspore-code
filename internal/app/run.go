package app

import "flag"

// Run parses CLI args, wires dependencies, and starts the application.
func Run(args []string) error {
	fs := flag.NewFlagSet("ms-cli", flag.ContinueOnError)
	demo := fs.Bool("demo", false, "Run in demo mode")
	configPath := fs.String("config", "", "Path to config file")
	url := fs.String("url", "", "OpenAI-compatible base URL")
	model := fs.String("model", "", "Model name")
	apiKey := fs.String("api-key", "", "API key")

	if err := fs.Parse(args); err != nil {
		return err
	}

	container, err := Wire(BootstrapConfig{
		Demo:       *demo,
		ConfigPath: *configPath,
		URL:        *url,
		Model:      *model,
		Key:        *apiKey,
	})
	if err != nil {
		return err
	}

	return container.App.Run()
}
