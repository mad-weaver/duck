package fileaction

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ actions.Action = (*FileAction)(nil)

type FileAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
	Params struct {
		Path  string  `mapstructure:"path" validate:"required"`
		Mode  *string `mapstructure:"mode"`
		Owner *string `mapstructure:"owner"`
		Group *string `mapstructure:"group"`
		Data  *string `mapstructure:"data"`
	} `mapstructure:"params"`
}

var configHelper = confighelper.GetConfigHelper()

func NewAction(ctx context.Context, konfig *koanf.Koanf) (*FileAction, error) {
	a := &FileAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load file action config: %w", err)
	}

	return a, nil
}

func (a *FileAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	// Write file contents if specified
	if a.Params.Data != nil {
		if err := os.WriteFile(a.Params.Path, []byte(*a.Params.Data), 0644); err != nil {
			return fmt.Errorf("failed to write file contents: %w", err)
		}
	}

	// Change mode if specified
	if a.Params.Mode != nil {
		mode, err := strconv.ParseUint(*a.Params.Mode, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode %s: %w", *a.Params.Mode, err)
		}
		if err := os.Chmod(a.Params.Path, os.FileMode(mode)); err != nil {
			return fmt.Errorf("failed to change file mode: %w", err)
		}
	}

	// Change owner/group if either is specified
	if a.Params.Owner != nil || a.Params.Group != nil {
		var uid, gid = -1, -1 // -1 means "don't change" in os.Chown

		// Resolve owner if specified
		if a.Params.Owner != nil {
			u, err := user.Lookup(*a.Params.Owner)
			if err != nil {
				return fmt.Errorf("failed to lookup user %s: %w", *a.Params.Owner, err)
			}
			uid, err = strconv.Atoi(u.Uid)
			if err != nil {
				return fmt.Errorf("invalid uid for user %s: %w", *a.Params.Owner, err)
			}
		}

		// Resolve group if specified
		if a.Params.Group != nil {
			g, err := user.LookupGroup(*a.Params.Group)
			if err != nil {
				return fmt.Errorf("failed to lookup group %s: %w", *a.Params.Group, err)
			}
			gid, err = strconv.Atoi(g.Gid)
			if err != nil {
				return fmt.Errorf("invalid gid for group %s: %w", *a.Params.Group, err)
			}
		}

		if err := os.Chown(a.Params.Path, uid, gid); err != nil {
			return fmt.Errorf("failed to change file ownership: %w", err)
		}
	}

	return nil
}

func (a *FileAction) GetConfig() actions.Config {
	return a.Config
}
