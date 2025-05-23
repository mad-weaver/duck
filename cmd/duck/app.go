// Package cmd is the main entrypoint for the checknrun CLI
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mad-weaver/duck/internal/sloghelper"
	"github.com/urfave/cli/v2"
)

// NewApp creates the CLI app object and sets the action directive to parse the checkfile into a koanf object
// and then pass that koanf object to the RunCheckFile function to process the checkfile.
func NewApp() *cli.App {

	app := cli.NewApp()
	app.Name = "duck"
	app.Usage = "Duck is a versatile task orchestration tool"
	app.UsageText = "duck -f <duckfile> [-t <target>] [options]"
	app.Flags = []cli.Flag{
		&cli.StringSliceFlag{
			Name:     "file",
			Aliases:  []string{"f"},
			Required: true,
			Usage:    "specify duckfile as path or URL (can be used multiple times)",
			EnvVars:  []string{"DUCK_FILE"},
		},
		&cli.StringFlag{
			Name:    "target",
			Aliases: []string{"t"},
			Value:   "default",
			Usage:   "specify target to run, default is 'default'",
			EnvVars: []string{"DUCK_TARGET"},
		},
		&cli.BoolFlag{
			Name:    "list-targets",
			Aliases: []string{"l"},
			Value:   false,
			Usage:   "list all available targets",
			EnvVars: []string{"DUCK_LIST_TARGETS"},
		},
		&cli.BoolFlag{
			Name:     "daemon",
			Aliases:  []string{"d"},
			Value:    false,
			Usage:    "enable daemon mode",
			EnvVars:  []string{"DUCK_DAEMON"},
			Category: "Daemon Control Options",
		},
		&cli.IntFlag{
			Name:     "daemon-timeout",
			Value:    0,
			Usage:    "terminate daemon after this many seconds, 0 means no timeout",
			EnvVars:  []string{"DUCK_DAEMON_TIMEOUT"},
			Category: "Daemon Control Options",
			Action: func(ctx *cli.Context, v int) error {
				if v < 0 {
					return fmt.Errorf("daemon-timeout must be greater than or equal to 0")
				}
				return nil
			},
		},
		&cli.IntFlag{
			Name:     "daemon-iterations",
			Value:    0,
			Usage:    "terminate daemon after this many runs, 0 means no limit",
			EnvVars:  []string{"DUCK_DAEMON_ITERATIONS"},
			Category: "Daemon Control Options",
			Action: func(ctx *cli.Context, v int) error {
				if v < 0 {
					return fmt.Errorf("daemon-iterations must be greater than or equal to 0")
				}
				return nil
			},
		},
		&cli.IntFlag{
			Name:     "daemon-interval",
			Aliases:  []string{"i"},
			Value:    60,
			Usage:    "time in seconds to pause between runs (default 60)",
			EnvVars:  []string{"DUCK_DAEMON_INTERVAL"},
			Category: "Daemon Control Options",
			Action: func(ctx *cli.Context, v int) error {
				if v < 0 {
					return fmt.Errorf("daemon-interval must be greater than 0")
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:     "loglevel",
			Value:    "info",
			Usage:    "specify log level (debug, info, warn, error)",
			EnvVars:  []string{"DUCK_LOGLEVEL"},
			Category: "Logging Options",
			Action: func(ctx *cli.Context, v string) error {
				if v != "debug" && v != "info" && v != "warn" && v != "error" {
					return fmt.Errorf("invalid log level: %s -- please use debug, info, warn, or error", v)
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:     "logformat",
			Value:    "text",
			Usage:    "specify log format (json, text, rich)",
			EnvVars:  []string{"DUCK_LOGFORMAT"},
			Category: "Logging Options",
			Action: func(ctx *cli.Context, v string) error {
				if v != "json" && v != "text" && v != "rich" {
					return fmt.Errorf("invalid log format: %s -- please use rich, json or text", v)
				}
				return nil
			},
		},
	}
	app.Before = func(c *cli.Context) error {
		// Create context that listens for interrupt signals
		ctx, stop := signal.NotifyContext(context.Background(),
			syscall.SIGTERM,
			syscall.SIGINT,
			os.Interrupt)

		// Store the stop func and context in metadata
		c.App.Metadata = map[string]interface{}{
			"stop": stop,
			"ctx":  ctx,
		}

		// Parse CLI args into koanf config - This will be implemented in parse_cli.go
		konfig, err := ParseCLI(c)
		if err != nil {
			return err
		}

		// Setup logging using the parsed config
		slog.SetDefault(sloghelper.SetupLoggerfromKoanf(konfig))

		return nil
	}
	app.After = func(c *cli.Context) error {
		// Clean up signal handling
		if stop, ok := c.App.Metadata["stop"].(func()); ok {
			stop()
		}
		return nil
	}
	app.HideHelpCommand = true
	app.Action = DefaultApp

	return app
}
