package duck

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/knadh/koanf/v2"

	"github.com/mad-weaver/duck/internal/confighelper"
	"github.com/mad-weaver/duck/internal/target"
)

const (
	ModifiedColon = "\u0A7A"
)

type Duck struct {
	Config    Config
	Duckfiles map[string]url.URL
	Targets   map[string]*target.Target
}

type Config struct {
	Files            []string `mapstructure:"file" validate:"required"`
	ListTargets      bool     `mapstructure:"list-targets" default:"false"`
	Target           string   `mapstructure:"target" default:"default"`
	Daemon           bool     `mapstructure:"daemon" default:"false"`
	DaemonInterval   int      `mapstructure:"daemon-interval" default:"60"`
	DaemonIterations int      `mapstructure:"daemon-iterations" default:"0"`
	DaemonTimeout    int      `mapstructure:"daemon-timeout" default:"0"`
	LogLevel         string   `mapstructure:"loglevel" default:"info"`
	LogFormat        string   `mapstructure:"logformat" default:"text"`
}

// NewDuck creates a new Duck object from a koanf object.
func NewDuck(k *koanf.Koanf) (*Duck, error) {

	//hydrate the configuration for a duck object from koanf.
	cfg := &Config{}
	cfghelper := confighelper.GetConfigHelper()
	if err := cfghelper.Load(cfg, k, "", "mapstructure"); err != nil {
		return nil, err
	}

	return &Duck{
		Config:    *cfg,
		Duckfiles: make(map[string]url.URL),
		Targets:   make(map[string]*target.Target),
	}, nil
}

// Run will compile the targets and run the target specified by the target name.
// It is the main execution function for duck.
func (d *Duck) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	slog.Debug("Compiling targets", "duck", d)
	if err := d.CompileTargets(ctx); err != nil {
		return err
	}

	if d.Config.ListTargets {
		return d.ListTargets(ctx)
	}

	return d.RunTarget(ctx, d.Config.Target, make(map[string]struct{}))
}
