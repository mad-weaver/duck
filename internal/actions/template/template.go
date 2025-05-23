package templateaction

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-resty/resty/v2"
	"github.com/knadh/koanf/v2"
	"github.com/mad-weaver/duck/internal/actions"
	"github.com/mad-weaver/duck/internal/confighelper"
	"gopkg.in/yaml.v3"
)

var _ actions.Action = (*TemplateAction)(nil)

type TemplateAction struct {
	Type   string         `mapstructure:"type"`
	Config actions.Config `mapstructure:"config"`
	Params struct {
		TemplateSource     string            `mapstructure:"template_source" validate:"required"`                          // URL or local file path for the template
		DataSource         string            `mapstructure:"data_source"`                                                  // Optional: URL, local file path for data, or raw data string
		IsDataSourceInline bool              `mapstructure:"is_data_source_inline" default:"false"`                        // If true, DataSource is raw data string, not a path/URL
		DataSourceFormat   string            `mapstructure:"data_source_format" default:"json" validate:"oneof=json yaml"` // "json", "yaml". Used if DataSource is not empty.
		OutputPath         string            `mapstructure:"output_path" validate:"required"`                              // Path to write the rendered output
		Headers            map[string]string `mapstructure:"headers" default:"{}"`                                         // Optional headers for fetching remote TemplateSource or DataSource (if not inline)
		InsecureSkipVerify bool              `mapstructure:"insecure_skip_verify" default:"false"`                         // For fetching remote sources
	} `mapstructure:"params"`
	client *resty.Client
}

var configHelper = confighelper.GetConfigHelper()

func NewAction(ctx context.Context, konfig *koanf.Koanf) (*TemplateAction, error) {
	a := &TemplateAction{}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before NewAction: %w", err)
	}

	if err := configHelper.Load(a, konfig, "", "mapstructure"); err != nil {
		return nil, fmt.Errorf("failed to load template action config: %w", err)
	}

	a.client = resty.New()
	if a.Params.InsecureSkipVerify {
		a.client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	}

	return a, nil
}

func (a *TemplateAction) fetchContent(source string) ([]byte, error) {
	u, err := url.Parse(source)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		slog.Debug("Fetching remote content", "url", source)
		req := a.client.R()
		if len(a.Params.Headers) > 0 {
			req.SetHeaders(a.Params.Headers)
		}
		resp, err := req.Get(source)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch remote content from %s: %w", source, err)
		}
		if resp.IsError() {
			return nil, fmt.Errorf("failed to fetch remote content from %s: status %s, body %s", source, resp.Status(), resp.String())
		}
		return resp.Body(), nil
	}
	slog.Debug("Reading local file content", "path", source)
	content, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("failed to read local file %s: %w", source, err)
	}
	return content, nil
}

func (a *TemplateAction) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	// Fetch template content
	slog.Debug("Fetching template", "source", a.Params.TemplateSource)
	templateContent, err := a.fetchContent(a.Params.TemplateSource)
	if err != nil {
		return fmt.Errorf("failed to get template content: %w", err)
	}

	// Prepare data map
	dataMap := make(map[string]interface{})
	if strings.TrimSpace(a.Params.DataSource) != "" {
		var dataSourceContent []byte
		if a.Params.IsDataSourceInline {
			slog.Debug("Using inline data source")
			dataSourceContent = []byte(a.Params.DataSource)
		} else {
			slog.Debug("Fetching data source", "source", a.Params.DataSource)
			dataSourceContent, err = a.fetchContent(a.Params.DataSource)
			if err != nil {
				return fmt.Errorf("failed to get data source content: %w", err)
			}
		}

		slog.Debug("Parsing data source", "format", a.Params.DataSourceFormat)
		switch strings.ToLower(a.Params.DataSourceFormat) {
		case "json":
			if err := json.Unmarshal(dataSourceContent, &dataMap); err != nil {
				return fmt.Errorf("failed to parse JSON data source: %w", err)
			}
		case "yaml":
			if err := yaml.Unmarshal(dataSourceContent, &dataMap); err != nil {
				return fmt.Errorf("failed to parse YAML data source: %w", err)
			}
		default:
			return fmt.Errorf("unsupported data source format: %s", a.Params.DataSourceFormat)
		}
	}

	// Parse and execute template
	slog.Debug("Parsing template", "template_name", filepath.Base(a.Params.TemplateSource))
	tmpl, err := template.New(filepath.Base(a.Params.TemplateSource)).Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var renderedOutput bytes.Buffer
	slog.Debug("Executing template")
	if err := tmpl.Execute(&renderedOutput, dataMap); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write output to file
	outputDir := filepath.Dir(a.Params.OutputPath)
	slog.Debug("Ensuring output directory exists", "path", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	slog.Debug("Writing rendered output to file", "path", a.Params.OutputPath)
	if err := os.WriteFile(a.Params.OutputPath, renderedOutput.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write output file %s: %w", a.Params.OutputPath, err)
	}

	slog.Info("Template rendered successfully", "output_path", a.Params.OutputPath)
	return nil
}

func (a *TemplateAction) GetConfig() actions.Config {
	return a.Config
}
