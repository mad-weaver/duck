package filecheck

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/checks"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ checks.Check = (*FileCheck)(nil)

type FileCheck struct {
	Type   string        `mapstructure:"type"`
	Status bool          `default:"false"`
	Config checks.Config `mapstructure:"config"`
	Params struct {
		Path     string            `mapstructure:"path" validate:"required"`
		Exists   bool              `mapstructure:"exists" default:"true"`
		Match    []string          `mapstructure:"match" default:"[]"`
		NoMatch  []string          `mapstructure:"no_match" default:"[]"`
		Metadata map[string]string `mapstructure:"metadata" default:"{}" validate:"dive,keys,oneof=owner group mode size modified_since,endkeys"`
	} `mapstructure:"params"`
}

var configHelper = confighelper.GetConfigHelper()

// NewCheck creates a new FileCheck. It takes a koanf object to
// hydrate the check struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewCheck(ctx context.Context, konfig *koanf.Koanf) (*FileCheck, error) {
	c := &FileCheck{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(c, konfig, "", "mapstructure"); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *FileCheck) Execute(ctx context.Context) error {
	// Check for context cancellation at start
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	f, err := os.Stat(c.Params.Path)

	// Check if file exists matches expectation
	if c.Params.Exists != !os.IsNotExist(err) {
		c.Status = false
		return nil
	}

	if len(c.Params.Match) > 0 || len(c.Params.NoMatch) > 0 {
		content, err := os.ReadFile(c.Params.Path)
		if err != nil {
			c.Status = false
			return fmt.Errorf("failed to read file: %w", err)
		}

		file_contents := string(content)
		for _, match := range c.Params.Match {
			if !strings.Contains(file_contents, match) {
				c.Status = false
				return nil
			}
		}
		for _, no_match := range c.Params.NoMatch {
			if strings.Contains(file_contents, no_match) {
				c.Status = false
				return nil
			}
		}
	}

	// Check file owner
	owner, ok := c.Params.Metadata["owner"]
	if ok && owner != "" {
		if stat, ok := f.Sys().(*syscall.Stat_t); ok {
			if strconv.Itoa(int(stat.Uid)) != owner {
				c.Status = false
				return nil
			}
		}
	}

	// Check file group
	group, ok := c.Params.Metadata["group"]
	if ok && group != "" {
		if stat, ok := f.Sys().(*syscall.Stat_t); ok {
			if strconv.Itoa(int(stat.Gid)) != group {
				c.Status = false
				return nil
			}
		}
	}

	// Check file permissions mode
	modeStr, ok := c.Params.Metadata["mode"]
	if ok && modeStr != "" {
		mode, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			c.Status = false
			return nil
		}
		if f.Mode()&os.FileMode(mode) != os.FileMode(mode) {
			c.Status = false
			return nil
		}
	}

	// Check minimum file size
	sizeStr, ok := c.Params.Metadata["size"]
	if ok && sizeStr != "" {
		sizeBytes, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			c.Status = false
			return nil
		}
		if f.Size() < sizeBytes {
			c.Status = false
			return nil
		}
	}

	// Check if file was modified within duration
	modifiedSinceStr, ok := c.Params.Metadata["modified_since"]
	if ok && modifiedSinceStr != "" {
		durationAgo, err := time.ParseDuration(modifiedSinceStr)
		if err != nil {
			c.Status = false
			return nil
		}
		cutOffTime := time.Now().Add(-durationAgo)
		if f.ModTime().Before(cutOffTime) {
			c.Status = false
			return nil
		}
	}

	// All checks passed
	c.Status = true
	return nil
}

func (c *FileCheck) Check() bool {
	return (c.Status != c.Config.Invert)
}

func (c *FileCheck) GetConfig() checks.Config {
	return c.Config
}
