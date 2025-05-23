package target

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/checks"
	"github.com/mad-weaver/duck/internal/confighelper"
)

type Target struct {
	Id           string           `mapstructure:"id"`
	Checks       []checks.Check   `mapstructure:"-"`
	Actions      []actions.Action `mapstructure:"-"`
	Cleared      bool             `default:"false"`
	Config       Config           `mapstructure:"config"`
	Dependencies []string         `mapstructure:"dependencies"`
	mu           sync.Mutex
}

type Config struct {
	CancelOnCheckFailure  *bool `mapstructure:"cancelOnCheckFailure"`
	CancelOnActionFailure *bool `mapstructure:"cancelOnActionFailure" default:"true"`
	ExitOnCheckFailure    *bool `mapstructure:"exitOnCheckFailure"`
	ExitOnActionFailure   *bool `mapstructure:"exitOnActionFailure"`
}

func NewTarget(ctx context.Context, k *koanf.Koanf) (*Target, error) {
	t := &Target{}

	slog.Debug("Creating target", "target", t)
	configHelper := confighelper.GetConfigHelper()
	if err := configHelper.Load(t, k, "", "mapstructure"); err != nil {
		return nil, err
	}

	slog.Debug("Loading checks", "target", t)
	for _, check := range k.Slices("checks") {
		check, err := t.LoadCheck(ctx, check)
		if err != nil {
			return nil, err
		}
		t.Checks = append(t.Checks, check)
	}

	slog.Debug("Loading actions", "target", t)
	for _, action := range k.Slices("actions") {
		action, err := t.LoadAction(ctx, action)
		if err != nil {
			return nil, err
		}
		t.Actions = append(t.Actions, action)
	}

	return t, nil
}

func (t *Target) Run(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	slog.Debug("Running target", "id", t.Id)

	if t.Cleared {
		slog.Debug("Target already run, skipping")
		return nil
	}

	if ctx.Err() != nil {
		slog.Debug("Context cancelled, skipping target", "id", t.Id)
		return fmt.Errorf("context cancelled, likely by termination signal/interrupt")
	}

	for _, check := range t.Checks {
		if err := check.Execute(ctx); err != nil {
			return err
		}

		chkcfg := check.GetConfig()
		// Check has failed, handle it.
		if !check.Check() {
			slog.Debug("Check failed")

			shouldExit := (chkcfg.ExitOnFailure != nil && *chkcfg.ExitOnFailure) ||
				(chkcfg.ExitOnFailure == nil && t.Config.ExitOnCheckFailure != nil && *t.Config.ExitOnCheckFailure)

			if shouldExit {
				slog.Debug("ExitOnCheckFailure set, terminating duck immediately", "id", t.Id)
				os.Exit(1)
			}

			shouldCancel := (chkcfg.CancelOnFailure != nil && *chkcfg.CancelOnFailure) ||
				(chkcfg.CancelOnFailure == nil && t.Config.CancelOnCheckFailure != nil && *t.Config.CancelOnCheckFailure)

			if shouldCancel {
				slog.Debug("Cancelling target", "id", t.Id)
				return fmt.Errorf("check failed, cancelling run")
			}

			slog.Debug("check failed, but no cancellation or exit set, moving to next target")
			t.Cleared = true
			return nil
		}

	}
	slog.Debug("all checks passed, executing actions")
	for _, action := range t.Actions {
		if err := action.Execute(ctx); err != nil {
			actioncfg := action.GetConfig()

			shouldExit := (actioncfg.ExitOnFailure != nil && *actioncfg.ExitOnFailure) ||
				(actioncfg.ExitOnFailure == nil && t.Config.ExitOnActionFailure != nil && *t.Config.ExitOnActionFailure)

			if shouldExit {
				slog.Debug("ExitOnActionFailure set, terminating duck immediately", "id", t.Id)
				os.Exit(1)
			}

			shouldCancel := (actioncfg.CancelOnFailure != nil && *actioncfg.CancelOnFailure) ||
				(actioncfg.CancelOnFailure == nil && t.Config.CancelOnActionFailure != nil && *t.Config.CancelOnActionFailure)

			if shouldCancel {
				slog.Debug("Cancelling target", "id", t.Id)
				return fmt.Errorf("action failed, cancelling run")
			}

			slog.Warn("Action failed, but no cancellation or exit set, Setting target to cleared and moving to next target", "id", t.Id)
			t.Cleared = true
			return nil
		}
	}
	slog.Debug("all actions passed, marking target cleared and moving onward.", "id", t.Id)
	t.Cleared = true
	return nil
}
