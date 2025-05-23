package shellcheck

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	"github.com/go-cmd/cmd"
	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/checks"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ checks.Check = (*ShellCheck)(nil)

type ShellCheck struct {
	Type   string        `mapstructure:"type"`
	Status bool          `default:"false"`
	Config checks.Config `mapstructure:"config"`
	Params struct {
		Command      string            `mapstructure:"command" default:"/bin/sh" validate:"required"`
		Args         []string          `mapstructure:"args" default:"[]"`
		ExitCode     int               `mapstructure:"exit_code" default:"0"`
		RegexMatch   []string          `mapstructure:"regex_match" default:"[]"`
		RegexNoMatch []string          `mapstructure:"regex_no_match" default:"[]"`
		Timeout      int               `mapstructure:"timeout" default:"20"`
		Env          map[string]string `mapstructure:"env" default:"{}"`
		NoInheritEnv bool              `mapstructure:"no_inherit_env" default:"false"`
		Echo         bool              `mapstructure:"echo" default:"false"`
		Dir          string            `mapstructure:"dir" default:""`
	} `mapstructure:"params"`
	command *cmd.Cmd
}

var configHelper = confighelper.GetConfigHelper()

func NewCheck(ctx context.Context, konfig *koanf.Koanf) (*ShellCheck, error) {
	c := &ShellCheck{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(c, konfig, "", "mapstructure"); err != nil {
		return nil, err
	}
	cmdOptions := cmd.Options{
		CombinedOutput: true,
	}

	c1 := cmd.NewCmdOptions(cmdOptions, c.Params.Command, c.Params.Args...)
	if c.Params.NoInheritEnv {
		c1.Env = []string{}
	} else {
		c1.Env = os.Environ()
	}

	if c.Params.Dir != "" {
		c1.Dir = c.Params.Dir
	}

	c.command = c1

	for key, value := range c.Params.Env {
		c1.Env = append(c1.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return c, nil
}
func (c *ShellCheck) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	if c.command.Status().Runtime > 0 {
		slog.Error("Command being erroneously retriggered. cancelling")
		return errors.New("command being erroneously retriggered. cancelling")
	}

	slog.Debug("Executing command", "command", c.Params.Command)
	sChan := c.command.Start()

	go func() {
		select {
		case <-time.After(time.Duration(c.Params.Timeout) * time.Second):
			slog.Error("Command timed out", "command", c.Params.Command)
			c.command.Stop()
			return
		case <-ctx.Done():
			slog.Debug("Command cancelled", "command", c.Params.Command)
			c.command.Stop()
			return
		}
	}()

	s1 := <-sChan
	slog.Debug("Command completed", "command", c.Params.Command)

	if s1.Error != nil {
		slog.Error("Command failed to run with error", "error", s1.Error)
		return s1.Error
	}

	if s1.Exit != c.Params.ExitCode {
		slog.Error("Command failed with exit code", "exit_code", s1.Exit)
		return nil
	}
	if len(c.Params.RegexMatch) > 0 {
		for _, value := range c.Params.RegexMatch {
			match, err := matchSlice(value, s1.Stdout)
			if err != nil {
				slog.Error("Error parsing regex value", "error", err)
				return err
			}
			if !match {
				slog.Debug("Regex match failed", "regex", value)
				return nil
			}
		}
	}
	// Check for negative regex matches
	if len(c.Params.RegexNoMatch) > 0 {
		for _, value := range c.Params.RegexNoMatch {
			match, err := matchSlice(value, s1.Stdout)
			if err != nil {
				slog.Error("Error parsing regex value", "error", err)
				return err
			}
			if match {
				slog.Debug("Negative regex match found", "regex", value)
				return nil
			}
		}
	}

	slog.Debug("Command completed successfully", "command", c.Params.Command)
	c.Status = true
	return nil
}

func matchSlice(regex string, items []string) (bool, error) {
	re, err := regexp.Compile(regex)
	if err != nil {
		return false, err
	}
	for _, value := range items {
		slog.Debug("trying to match regex with line", "output", value, "regex", regex)
		if re.MatchString(value) {
			return true, nil
		}
	}
	return false, nil
}

func (c *ShellCheck) Check() bool {
	return (c.Status != c.Config.Invert)
}

func (c *ShellCheck) GetConfig() checks.Config {
	return c.Config
}
