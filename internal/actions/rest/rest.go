package restaction

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
)

var _ actions.Action = (*RestAction)(nil)

type RestAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
	Params struct {
		Method        string            `mapstructure:"method" default:"GET" validate:"http_method"`
		URL           string            `mapstructure:"url" validate:"required"`
		BasicUsername string            `mapstructure:"username" validate:"omitempty,required_with=BasicPassword"`
		BasicPassword string            `mapstructure:"password" validate:"omitempty,required_with=BasicUsername"`
		Headers       map[string]string `mapstructure:"headers" default:"{}"`
		Body          string            `mapstructure:"body"`
		Timeout       int               `mapstructure:"timeout" default:"20" validate:"omitempty,min=0"` // Timeout in seconds
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

func NewAction(ctx context.Context, konfig *koanf.Koanf) (*RestAction, error) {
	a := &RestAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load rest action config: %w", err)
	}

	a.client = resty.New()

	return a, nil
}

// configureTLS sets up TLS configuration for the REST client
func (a *RestAction) configureTLS(client *resty.Client) error {
	if a.Params.TLS.InsecureSkipVerify {
		client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
		return nil
	}

	if a.Params.TLS.CertFile == "" && a.Params.TLS.KeyFile == "" && a.Params.TLS.CAFile == "" {
		return nil
	}

	tlsConfig := &tls.Config{}

	// Load client certificates if specified
	if a.Params.TLS.CertFile != "" && a.Params.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(a.Params.TLS.CertFile, a.Params.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificates if specified
	if a.Params.TLS.CAFile != "" {
		caCert, err := os.ReadFile(a.Params.TLS.CAFile)
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

func (a *RestAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	// Configure timeout if specified
	if a.Params.Timeout > 0 {
		a.client.SetTimeout(time.Duration(a.Params.Timeout) * time.Second)
	}

	// Configure TLS settings
	if err := a.configureTLS(a.client); err != nil {
		return err
	}

	// Create request with context
	resp := a.client.R().SetContext(ctx)

	// Set basic auth if both username and password are provided
	if a.Params.BasicUsername != "" && a.Params.BasicPassword != "" {
		resp.SetBasicAuth(a.Params.BasicUsername, a.Params.BasicPassword)
	}

	// Set content type if specified
	if a.Params.ContentType != "" {
		resp.SetHeader("Content-Type", a.Params.ContentType)
	}

	// Set additional headers if specified
	if len(a.Params.Headers) > 0 {
		for header, value := range a.Params.Headers {
			resp.SetHeader(header, value)
		}
	}

	// Set body if specified
	if a.Params.Body != "" {
		resp.SetBody(a.Params.Body)
	}

	// Execute the request using the method map
	method := strings.ToUpper(a.Params.Method)
	fn, ok := methodMap[method]
	if !ok {
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	response, err := fn(resp, a.Params.URL)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}

	slog.Debug("Rest call returned", "status_code", response.StatusCode)

	return nil
}

func (a *RestAction) GetConfig() actions.Config {
	return a.Config
}
