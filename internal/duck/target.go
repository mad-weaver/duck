package duck

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/target"
)

// appendTarget will unmarshal a koanf object into a target object and append it to the duck Target map.
// accepts a context, a target name, and a koanf object. sets target ID and its map key to the "name" parameter.
func (d *Duck) appendTarget(ctx context.Context, name string, konfig *koanf.Koanf) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	if _, exists := d.Targets[name]; exists {
		return fmt.Errorf("target %s already exists", name)
	}
	konfig.Set("id", name)

	target, err := target.NewTarget(ctx, konfig)
	if err != nil {
		return fmt.Errorf("failed to create target %s: %w", name, err)
	}

	slog.Debug("appending target", "name", name, "konfig", konfig)
	d.Targets[name] = target
	return nil
}

// ListTargets lists all the targets in the duck object. If targets are not compiled, it will call a compile.
// accepts a context, only affects internal state of duck object.
func (d *Duck) ListTargets(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	if len(d.Targets) == 0 {
		if err := d.CompileTargets(ctx); err != nil {
			return err
		}
	}

	for target := range d.Targets {
		fmt.Println(target)
	}

	return nil
}

// RunTarget will run the target specified by the target name.
// accepts a context, a target name, and a lineage map. lineage is a hash
// of all targets that are scheduled to be executed and is used to detect loops
// and avoid scheduling them. If a Target has dependent targets, it will add itself to
// the lineage and then recursively call each depdendent target. Assumes CompileTargets
// was called at some point before running this else this will fail.
func (d *Duck) RunTarget(ctx context.Context, target string, lineage map[string]struct{}) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	// check if the target exists
	if _, exists := d.Targets[target]; !exists {
		return fmt.Errorf("target %s not found", target)
	}

	// check if the target is already in the lineage
	if _, exists := lineage[target]; exists {
		slog.Debug("target already in enqueued, skipping to avoid loops", "target", target)
		return nil
	}

	// check if the target is already cleared
	if d.Targets[target].Cleared {
		slog.Debug("target already cleared, skipping", "target", target)
		return nil
	}

	slog.Debug("running target", "target", target)

	// add this target to the lineage
	lineage[target] = struct{}{}

	// check if the target has dependencies
	if len(d.Targets[target].Dependencies) > 0 {
		for _, dependency := range d.Targets[target].Dependencies {
			if err := d.RunTarget(ctx, dependency, lineage); err != nil {
				return err
			}
		}
	}

	// run the target
	return d.Targets[target].Run(ctx)
}
