// Package confighelper is a helper space for loading the validator singleton,
// making pretty validation errors, and loading koanf into structs.

package confighelper

import (
	"fmt"
	"strings"
	"sync"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/v2"
)

// ConfigHelper handles configuration loading with caching of validators
type ConfigHelper struct {
	validate *validator.Validate
	mu       sync.RWMutex
}

var (
	defaultLoader *ConfigHelper
	once          sync.Once
)

// formatValidationErrors converts validator.ValidationErrors into a more user-friendly format
func formatValidationErrors(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var messages []string
		for _, e := range validationErrors {
			field := e.Field()
			tag := e.Tag()
			param := e.Param()

			switch tag {
			case "required":
				messages = append(messages, fmt.Sprintf("field '%s' is required", field))
			case "oneof":
				messages = append(messages, fmt.Sprintf("field '%s' must be one of [%s]", field, param))
			case "gt":
				messages = append(messages, fmt.Sprintf("field '%s' must be greater than %s", field, param))
			case "gte":
				messages = append(messages, fmt.Sprintf("field '%s' must be greater than or equal to %s", field, param))
			case "lt":
				messages = append(messages, fmt.Sprintf("field '%s' must be less than %s", field, param))
			case "lte":
				messages = append(messages, fmt.Sprintf("field '%s' must be less than or equal to %s", field, param))
			case "min":
				messages = append(messages, fmt.Sprintf("field '%s' must have a minimum value of %s", field, param))
			case "max":
				messages = append(messages, fmt.Sprintf("field '%s' must have a maximum value of %s", field, param))
			case "required_with":
				messages = append(messages, fmt.Sprintf("field '%s' is required when %s is present", field, param))
			case "file":
				messages = append(messages, fmt.Sprintf("field '%s' must be a valid file path", field))
			case "http_method":
				messages = append(messages, fmt.Sprintf("field '%s' must be a valid HTTP method", field))
			case "octal":
				messages = append(messages, fmt.Sprintf("field '%s' must be a valid octal number", field))
			default:
				messages = append(messages, fmt.Sprintf("field '%s' failed validation: %s=%s", field, tag, param))
			}
		}
		return fmt.Sprintf("Configuration validation failed:\n  - %s", strings.Join(messages, "\n  - "))
	}
	return err.Error()
}

// GetConfigLoader returns the singleton instance of ConfigLoader
func GetConfigHelper() *ConfigHelper {
	once.Do(func() {
		defaultLoader = &ConfigHelper{
			validate: validator.New(),
		}
	})
	return defaultLoader
}

// Load is a helper function to hydrate an interface/struct (config)from a koanf(konfig)
// Path(path) is the key to inspect to unmarshal into the config struct. Set to "" to unmarshal
// the entire config struct. marshalTag is the struct tag to use to match against for unmarshalling.
// You probably want to use something like "mapstructure"
func (cl *ConfigHelper) Load(config interface{}, konfig *koanf.Koanf, path string, marshalTag string) error {

	// Set defaults from struct tags "default"
	if err := defaults.Set(config); err != nil {
		return fmt.Errorf("error setting defaults: %w", err)
	}

	// Unmarshal configuration from koanf into the config struct
	if err := konfig.UnmarshalWithConf(path, config, koanf.UnmarshalConf{
		Tag: marshalTag,
	}); err != nil {
		return fmt.Errorf("error unmarshalling config: %w", err)
	}

	// Validate using cached validator
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	if err := cl.validate.Struct(config); err != nil {
		return fmt.Errorf("%s", formatValidationErrors(err))
	}

	return nil
}

// RegisterValidation registers a custom validation function
func (cl *ConfigHelper) RegisterValidation(tag string, fn validator.Func) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.validate.RegisterValidation(tag, fn)
}
