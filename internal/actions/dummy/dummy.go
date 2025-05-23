package dummyaction

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ actions.Action = (*DummyAction)(nil)

type DummyAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
}

var configHelper = confighelper.GetConfigHelper()

// NewAction creates a new DummyAction. It takes a koanf object to
// hydrate the action struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewAction(ctx context.Context, konfig *koanf.Koanf) (*DummyAction, error) {
	a := &DummyAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, err
	}

	return a, nil
}

func (a *DummyAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	return nil
}

func (a *DummyAction) GetConfig() actions.Config {
	return a.Config
}
