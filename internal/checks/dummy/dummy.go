package dummycheck

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/checks"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ checks.Check = (*DummyCheck)(nil)

type DummyCheck struct {
	Type   string        `mapstructure:"type"`
	Status bool          `default:"false"`
	Config checks.Config `mapstructure:"config"`
}

var configHelper = confighelper.GetConfigHelper()

// NewCheck creates a new DummyCheck. It takes a koanf object to
// hydrate the check struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewCheck(ctx context.Context, konfig *koanf.Koanf) (*DummyCheck, error) {
	c := &DummyCheck{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(c, konfig, "", "mapstructure"); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *DummyCheck) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	c.Status = true
	return nil
}

func (c *DummyCheck) Check() bool {
	return (c.Status != c.Config.Invert)
}

func (c *DummyCheck) GetConfig() checks.Config {
	return c.Config
}
