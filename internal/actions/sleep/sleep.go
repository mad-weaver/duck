package sleepaction

import (
	"context"
	"fmt"
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ actions.Action = (*SleepAction)(nil)

type SleepAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
	Params struct {
		Seconds int `mapstructure:"seconds" validate:"required,min=0"`
	} `mapstructure:"params"`
}

var configHelper = confighelper.GetConfigHelper()

// NewAction creates a new SleepAction. It takes a koanf object to
// hydrate the action struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewAction(ctx context.Context, konfig *koanf.Koanf) (*SleepAction, error) {
	a := &SleepAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load sleep action config: %w", err)
	}

	return a, nil
}

func (a *SleepAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	select {
	case <-time.After(time.Duration(a.Params.Seconds) * time.Second):
		// Sleep completed
	case <-ctx.Done():
		return fmt.Errorf("sleep interrupted: %w", ctx.Err())
	}

	return nil
}

func (a *SleepAction) GetConfig() actions.Config {
	return a.Config
}
