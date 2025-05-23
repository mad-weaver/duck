package croncheck

import (
	"context"
	"fmt"
	"time"

	"github.com/adhocore/gronx"
	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/checks"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ checks.Check = (*CronCheck)(nil)

type CronCheck struct {
	Type   string        `mapstructure:"type"`
	Status bool          `default:"false"`
	Config checks.Config `mapstructure:"config"`
	Params struct {
		Expression string `mapstructure:"expression" validate:"required" default:"* * * * * * *"`
		Timezone   string `mapstructure:"timezone" default:"UTC"`
	} `mapstructure:"params"`

	gronx *gronx.Gronx   // unexported
	tz    *time.Location // unexported
}

var configHelper = confighelper.GetConfigHelper()

// NewCheck creates a new CronCheck. It takes a koanf object to
// hydrate the check struct. It consumes the whole koanf object, so you likely want to
// carve it off a larger koanf object.
func NewCheck(ctx context.Context, konfig *koanf.Koanf) (*CronCheck, error) {
	c := &CronCheck{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(c, konfig, "", "mapstructure"); err != nil {
		return nil, err
	}

	// Initialize timezone
	if c.Params.Timezone != "" {
		loc, err := time.LoadLocation(c.Params.Timezone)
		if err != nil {
			return nil, fmt.Errorf("failed to load timezone %q: %w", c.Params.Timezone, err)
		}
		c.tz = loc
	} else {
		// Default to UTC if no timezone is specified
		c.tz = time.UTC
	}

	// Initialize gronx parser
	c.gronx = gronx.New()

	// Validate the cron expression
	if !c.gronx.IsValid(c.Params.Expression) {
		return nil, fmt.Errorf("invalid cron expression: %q", c.Params.Expression)
	}

	return c, nil
}

func (c *CronCheck) Execute(ctx context.Context) error {
	// Check for context cancellation at start
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	t := time.Now().In(c.tz)
	status, err := c.gronx.IsDue(c.Params.Expression, t)
	if err != nil {
		return fmt.Errorf("failed to check cron expression: %w", err)
	}

	c.Status = status

	return nil
}

func (c *CronCheck) Check() bool {
	return (c.Status != c.Config.Invert)
}

func (c *CronCheck) GetConfig() checks.Config {
	return c.Config
}
