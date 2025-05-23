package cmd

import (
	"context"
	"log/slog"
	"time"

	"github.com/mad-weaver/duck/internal/duck"
	"github.com/urfave/cli/v2"
)

func DefaultApp(c *cli.Context) error {
	ctx := c.App.Metadata["ctx"].(context.Context)
	konfig, err := ParseCLI(c)
	if err != nil {
		return err
	}

	running := true
	iterationCount := 0
	var timeoutCh <-chan time.Time

	exitch := make(chan struct{})
	go func() {
		<-exitch
		if stop, ok := c.App.Metadata["stop"].(func()); ok {
			stop()
		}
	}()

	for running {
		select {
		case <-ctx.Done():
			slog.Info("Received interrupt signal")
			running = false
		default:
			d, err := duck.NewDuck(konfig.Copy())
			if err != nil {
				return err
			}
			err = d.Run(ctx)
			if err != nil {
				return err
			}

			if konfig.Bool("daemon") {
				if timeoutCh == nil && konfig.Int("daemon-timeout") > 0 {
					timeoutCh = time.After(time.Duration(konfig.Int("daemon-timeout")) * time.Second)
				}

				slog.Debug("Target Run completed, sleeping for specififed interval before next run", "interval", konfig.Int("daemon-interval"))
				select {
				case <-ctx.Done():
					slog.Info("Received interrupt signal, terminating")
					running = false
				case <-timeoutCh:
					slog.Info("Daemon timeout reached, terminating")
					exitch <- struct{}{}
					running = false
				case <-time.After(time.Duration(konfig.Int("daemon-interval")) * time.Second):
					slog.Debug("Target Run completed, sleeping for specififed interval before next run", "interval", konfig.Int("daemon-interval"))
					iterationCount++
					if konfig.Int("daemon-iterations") > 0 && iterationCount >= konfig.Int("daemon-iterations") {
						slog.Info("Reached maximum number of iterations, terminating")
						exitch <- struct{}{}
						running = false
					}

				}
			} else {
				running = false
			}
		}
	}

	return nil
}
