package shellaction

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-cmd/cmd"
	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ actions.Action = (*ShellAction)(nil)

type ShellAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
	Params struct {
		Command      string            `mapstructure:"command" default:"/bin/sh" validate:"required"`
		Args         []string          `mapstructure:"args" default:"[]"`
		Timeout      int               `mapstructure:"timeout" default:"20"`
		Env          map[string]string `mapstructure:"env" default:"{}"`
		NoInheritEnv bool              `mapstructure:"no_inherit_env" default:"false"`
		Dir          string            `mapstructure:"dir" default:""`
		Echo         bool              `mapstructure:"echo" default:"false"`
	} `mapstructure:"params"`
	command *cmd.Cmd
}

var configHelper = confighelper.GetConfigHelper()

// NewAction creates a new ShellAction. It takes a koanf object to
// hydrate the action struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewAction(ctx context.Context, konfig *koanf.Koanf) (*ShellAction, error) {
	a := &ShellAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load shell action config: %w", err)
	}

	cmdOptions := cmd.Options{
		CombinedOutput: true,
	}

	if strings.HasPrefix(a.Params.Command, "/bin/") && strings.HasSuffix(a.Params.Command, "sh") {
		a.Params.Args = append([]string{"-c"}, a.Params.Args...)
	}

	c1 := cmd.NewCmdOptions(cmdOptions, a.Params.Command, a.Params.Args...)
	if a.Params.NoInheritEnv {
		c1.Env = []string{}
	} else {
		c1.Env = os.Environ()
	}

	if a.Params.Dir != "" {
		c1.Dir = a.Params.Dir
	}

	a.command = c1

	for key, value := range a.Params.Env {
		c1.Env = append(c1.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return a, nil
}

func (a *ShellAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	slog.Debug("Executing command", "command", a.Params.Command)
	sChan := a.command.Start()

	go func() {
		select {
		case <-time.After(time.Duration(a.Params.Timeout) * time.Second):
			slog.Error("Command timed out", "command", a.Params.Command)
			a.command.Stop()
			return
		case <-ctx.Done():
			slog.Debug("Command cancelled", "command", a.Params.Command)
			a.command.Stop()
			return
		}
	}()

	s1 := <-sChan
	slog.Debug("Command completed", "command", a.Params.Command)

	if a.Params.Echo && len(s1.Stdout) > 0 {
		fmt.Println(strings.Join(s1.Stdout, "\n"))
	}

	if s1.Error != nil {
		slog.Error("Command failed to run with error", "error", s1.Error)
		return s1.Error
	}

	if s1.Exit != 0 {
		slog.Debug("Command exited with non-zero status", "exit_code", s1.Exit)
	}

	return nil
}

func (a *ShellAction) GetConfig() actions.Config {
	return a.Config
}
