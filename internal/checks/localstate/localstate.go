package localstatecheck

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/checks"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ checks.Check = (*LocalStateCheck)(nil)

type LocalStateCheck struct {
	Type   string        `mapstructure:"type"`
	Status bool          `default:"false"`
	Config checks.Config `mapstructure:"config"`
	Params struct {
		Path     string   `mapstructure:"path" default:"/var/lib/duck/states" validate:"required"`
		IdPrefix string   `mapstructure:"id_prefix" default:"_localstate_"`
		Id       string   `mapstructure:"id" validate:"required"`
		Matches  []string `mapstructure:"matches" default:"[]" validate:"required"`
	} `mapstructure:"params"`
}

var configHelper = confighelper.GetConfigHelper()

func NewCheck(ctx context.Context, konfig *koanf.Koanf) (*LocalStateCheck, error) {
	c := &LocalStateCheck{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(c, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load local state check config: %w", err)
	}

	return c, nil
}

func (c *LocalStateCheck) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	filePath := filepath.Join(c.Params.Path, c.Params.IdPrefix+c.Params.Id)

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		// Error reading file. Check if this is a null state check.
		if len(c.Params.Matches) == 0 {
			if os.IsNotExist(err) {
				// File does not exist, which is the expected null state.
				slog.Debug("State file does not exist -- null state")
				if len(c.Params.Matches) == 0 {
					c.Status = true
				}
				return nil
			}
		}
		// Regular check: Matches is not empty, but file could not be read.
		return fmt.Errorf("failed to read state file %s: %w", filePath, err)
	}

	// File was read successfully. Now check based on Matches content.
	if len(c.Params.Matches) == 0 {
		// Null state check, but file exists. This is an error.
		slog.Debug("State file exists, matching to null state failed")
		return nil
	}

	// Regular check: Matches is not empty, file exists, now check content.
	fileContent := strings.TrimSpace(string(contentBytes))

	for _, matchString := range c.Params.Matches {
		if fileContent == matchString {
			c.Status = true
		}
	}

	return nil // Match found
}

func (c *LocalStateCheck) Check() bool {
	return (c.Status != c.Config.Invert)
}

func (c *LocalStateCheck) GetConfig() checks.Config {
	return c.Config
}
