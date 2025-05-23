package restcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/checks"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ checks.Check = (*RestCheck)(nil)

type RestCheck struct {
	Type   string        `mapstructure:"type"`
	Status bool          `default:"false"`
	Config checks.Config `mapstructure:"config"`
	Params struct {
		Method        string            `mapstructure:"method" default:"GET" validate:"http_method"`
		URL           string            `mapstructure:"url" validate:"required"`
		BasicUsername string            `mapstructure:"username" validate:"omitempty,required_with=BasicPassword"`
		BasicPassword string            `mapstructure:"password" validate:"omitempty,required_with=BasicUsername"`
		Headers       map[string]string `mapstructure:"headers" default:"{}"`
		Body          string            `mapstructure:"body"`
		Matches       []string          `mapstructure:"matches" default:"[]"`
		ExpectCode    int               `mapstructure:"expectCode" default:"200" validate:"gte=0,lt=600"`
		Timeout       int               `mapstructure:"timeout" validate:"omitempty,min=0"` // Timeout in seconds
		ContentType   string            `mapstructure:"content_type" default:"application/json"`
		TLS           struct {
			InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify" default:"false"`
			CertFile           string `mapstructure:"cert_file" validate:"omitempty,file"`
			KeyFile            string `mapstructure:"key_file" validate:"omitempty,file"`
			CAFile             string `mapstructure:"ca_file" validate:"omitempty,file"`
		} `mapstructure:"tls"`
	} `mapstructure:"params"`
	client *resty.Client
}

var configHelper = confighelper.GetConfigHelper()

// Map of supported HTTP methods to their corresponding resty client functions
var methodMap = map[string]func(*resty.Request, string) (*resty.Response, error){
	"GET":     (*resty.Request).Get,
	"POST":    (*resty.Request).Post,
	"PUT":     (*resty.Request).Put,
	"DELETE":  (*resty.Request).Delete,
	"PATCH":   (*resty.Request).Patch,
	"HEAD":    (*resty.Request).Head,
	"OPTIONS": (*resty.Request).Options,
}

func NewCheck(ctx context.Context, konfig *koanf.Koanf) (*RestCheck, error) {
	c := &RestCheck{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(c, konfig, "", "mapstructure"); err != nil {
		return nil, err
	}

	c.client = resty.New()

	return c, nil
}

// configureTLS sets up TLS configuration for the REST client
func (c *RestCheck) configureTLS(client *resty.Client) error {
	if c.Params.TLS.InsecureSkipVerify {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		return nil
	}

	if c.Params.TLS.CertFile == "" && c.Params.TLS.KeyFile == "" && c.Params.TLS.CAFile == "" {
		return nil
	}

	tlsConfig := &tls.Config{}

	// Load client certificates if specified
	if c.Params.TLS.CertFile != "" && c.Params.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.Params.TLS.CertFile, c.Params.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificates if specified
	if c.Params.TLS.CAFile != "" {
		caCert, err := os.ReadFile(c.Params.TLS.CAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	client.SetTLSClientConfig(tlsConfig)
	return nil
}

func (c *RestCheck) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	// Configure timeout if specified
	if c.Params.Timeout > 0 {
		c.client.SetTimeout(time.Duration(c.Params.Timeout) * time.Second)
	}

	// Configure TLS settings
	if err := c.configureTLS(c.client); err != nil {
		return err
	}

	// Create request with context
	resp := c.client.R().SetContext(ctx)

	// Set basic auth if both username and password are provided
	if c.Params.BasicUsername != "" && c.Params.BasicPassword != "" {
		resp.SetBasicAuth(c.Params.BasicUsername, c.Params.BasicPassword)
	}

	// Set content type if specified
	if c.Params.ContentType != "" {
		resp.SetHeader("Content-Type", c.Params.ContentType)
	}

	// Set additional headers if specified
	if len(c.Params.Headers) > 0 {
		for header, value := range c.Params.Headers {
			resp.SetHeader(header, value)
		}
	}

	// Set body if specified
	if c.Params.Body != "" {
		resp.SetBody(c.Params.Body)
	}

	// Execute the request using the method map
	method := strings.ToUpper(c.Params.Method)
	fn, ok := methodMap[method]
	if !ok {
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	response, err := fn(resp, c.Params.URL)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}

	if response.StatusCode() != c.Params.ExpectCode && c.Params.ExpectCode != 0 {
		return fmt.Errorf("unexpected status code: got %d, want %d", response.StatusCode(), c.Params.ExpectCode)
	}

	if len(c.Params.Matches) > 0 {
		body := response.String()
		for _, match := range c.Params.Matches {
			if !strings.Contains(body, match) {
				return fmt.Errorf("response body does not contain expected string: %s", match)
			}
		}
	}

	c.Status = true
	return nil
}

func (c *RestCheck) Check() bool {
	return (c.Status != c.Config.Invert)
}

func (c *RestCheck) GetConfig() checks.Config {
	return c.Config
}
