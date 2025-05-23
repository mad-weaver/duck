package actions

import "context"

type Action interface {
	Execute(context.Context) error // Runs the Action
	GetConfig() Config             // Returns the Action's configuration
}

type Config struct {
	CancelOnFailure *bool `mapstructure:"cancelOnFailure"`
	ExitOnFailure   *bool `mapstructure:"exitOnFailure"`
}
