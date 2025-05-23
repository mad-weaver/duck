package localstateaction

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ actions.Action = (*LocalStateAction)(nil)

type LocalStateAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
	Params struct {
		Path      string `mapstructure:"path" default:"/var/lib/duck/states" validate:"required"`
		IdPrefix  string `mapstructure:"id_prefix" default:"_localstate_"`
		Id        string `mapstructure:"id" validate:"required"`
		State     string `mapstructure:"state"`
		WipeState bool   `mapstructure:"wipe_state" default:"false"`
	} `mapstructure:"params"`
}

var configHelper = confighelper.GetConfigHelper()

// NewAction creates a new LocalStateAction. It takes a koanf object to
// hydrate the action struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewAction(ctx context.Context, konfig *koanf.Koanf) (*LocalStateAction, error) {
	a := &LocalStateAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load local state action config: %w", err)
	}

	return a, nil
}

func (a *LocalStateAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	filePath := filepath.Join(a.Params.Path, a.Params.IdPrefix+a.Params.Id)

	if a.Params.WipeState {
		slog.Debug("Removing state file", "path", filePath)
		err := os.Remove(filePath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove state file %s: %w", filePath, err)
		}
		slog.Debug("State file removed or did not exist", "path", filePath)
	} else {
		// Ensure directory exists
		dirPath := filepath.Dir(filePath)
		slog.Debug("Ensuring state directory exists", "path", dirPath)
		err := os.MkdirAll(dirPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create state directory %s: %w", dirPath, err)
		}

		slog.Debug("Writing state to file", "path", filePath)
		err = os.WriteFile(filePath, []byte(a.Params.State), 0644)
		if err != nil {
			return fmt.Errorf("failed to write state file %s: %w", filePath, err)
		}
		slog.Debug("State written to file", "path", filePath)
	}

	return nil
}

func (a *LocalStateAction) GetConfig() actions.Config {
	return a.Config
}
