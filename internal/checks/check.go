package checks

import "context"

type Check interface {
	Execute(context.Context) error // Runs the Check
	Check() bool                   // Returns the state of the Check
	GetConfig() Config             // Returns the Check's configuration
}

type Config struct {
	Invert          bool  `default:"false"`
	CancelOnFailure *bool `mapstructure:"cancelOnFailure"`
	ExitOnFailure   *bool `mapstructure:"exitOnFailure"`
}
