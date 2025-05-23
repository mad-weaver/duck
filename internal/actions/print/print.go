package printaction

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ actions.Action = (*PrintAction)(nil)

type PrintAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
	Params struct {
		Message string `mapstructure:"message"`
	} `mapstructure:"params"`
}

var configHelper = confighelper.GetConfigHelper()

// NewAction creates a new PrintAction. It takes a koanf object to
// hydrate the action struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewAction(ctx context.Context, konfig *koanf.Koanf) (*PrintAction, error) {
	a := &PrintAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load print action config: %w", err)
	}

	return a, nil
}

func (a *PrintAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	fmt.Println(a.Params.Message)

	return nil
}

func (a *PrintAction) GetConfig() actions.Config {
	return a.Config
}
