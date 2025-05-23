package cmd

import (
	"slices"
	"strings"

	kenv "github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	urfave "github.com/mad-weaver/duck/internal/urfave_provider"
	"github.com/urfave/cli/v2"
)

const (
	// ModifiedColon (꞉) is used as a delimiter that won't conflict with URLs or file paths
	ModifiedColon = "\u0A7A"
)

// ParseCLI is a wrapper function that handles converting the command invocation from CLI and returns it as a koanf object. It will
// load environment variables prefixed with "MYPROG_" unless they are handled by urfave.
//
// Args:
// ctx -> urfave/cli context
func ParseCLI(ctx *cli.Context) (*koanf.Koanf, error) {
	konfig := koanf.New(ModifiedColon)

	// exclude any envvar handled by urfave, this only handles miscellaneous stuff you might want
	// to put into the base koanf object
	excludedEnvVars := []string{
		"DUCK_DAEMON",
		"DUCK_DAEMON_TIMEOUT",
		"DUCK_DAEMON_ITERATIONS",
		"DUCK_DAEMON_INTERVAL",
		"DUCK_LOGLEVEL",
		"DUCK_LOGFORMAT",
		"DUCK_FILE",
		"DUCK_TARGET",
		"DUCK_EXIT_ON_CHECK_FAIL",
		"DUCK_EXIT_ON_ACTION_FAIL",
		"DUCK_CANCEL_ON_CHECK_FAIL",
		"DUCK_CANCEL_ON_ACTION_FAIL",
		"DUCK_LIST_TARGETS",
	}

	// push environment variables prefixed with DUCK_ into koanf object
	if err := konfig.Load(kenv.Provider("DUCK_", ModifiedColon, func(s string) string {
		if slices.Contains(excludedEnvVars, s) {
			return "" // Return an empty string to skip this variable
		}
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "DUCK_")), "_", ".", -1)
	}), nil); err != nil {
		return nil, err
	}

	// Push CLI args into koanf object
	forcedInclude := []string{"loglevel", "list-targets", "logformat", "daemon", "daemon-timeout", "daemon-iterations", "daemon-interval", "target", "file"}
	if err := konfig.Load(urfave.NewUrfaveCliProvider(ctx, konfig, ModifiedColon, false, forcedInclude), nil); err != nil {
		return nil, err
	}

	return konfig, nil
}
