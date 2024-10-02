package main

import (
	"github.com/peerdns/peerdns/cmd"
	"github.com/peerdns/peerdns/pkg/config"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"log"
	"os"
)

func main() {
	app := &cli.App{
		Name:  "peerdns",
		Usage: "To be defined...",
		Commands: []*cli.Command{
			cmd.KeystoreCommand(),
			cmd.CertsCommand(), // Command for handling certificates
			cmd.RuntimeCommand(),
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Config file path",
				Value:   "./config.yaml",
			},
		},
		Before: func(c *cli.Context) error {
			// Initialize global configuration. Can be accessed later on via config.G()
			cfg, err := config.InitializeGlobalConfig(c.String("config"))
			if err != nil {
				return errors.Wrap(err, "failure to load main peerdns configuration file")
			}

			// Initialize the global logger. Can be accessed later on via logger.G()
			gLog, err := logger.InitializeGlobalLogger(cfg.Logger)
			if err != nil {
				return errors.Wrap(err, "failure to initialize logger")
			}

			gLog.Debug(
				"Successfully loaded global configuration and logger setup",
				zap.String("environment", cfg.Logger.Environment),
				zap.String("level", cfg.Logger.Level),
			)

			return nil
		},
		After: func(c *cli.Context) error {
			return logger.Sync()
		},
	}

	// Run the app and handle any errors
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("failure while running peerdns: %v", err)
	}
}
