package duck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	"gocloud.dev/blob"

	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
)

// CompileTargets will compile the targets from the duckfiles specified when the
// constructor was called for duck. accepts a context, only affects internal state of duck object.
func (d *Duck) CompileTargets(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	for _, duckfile := range d.Config.Files {
		duckfiles, err := GetDuckfiles(ctx, duckfile)
		if err != nil {
			return err
		}

		for _, duckfile := range duckfiles {
			if err := d.LoadDuckfile(ctx, duckfile, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadDuckfile will load a duckfile into the duck object.
// accepts a context, a duckfile url, and a recurse bool. recurse is used to
// signal if the duckfile is loaded in a manner that will also load any dependencies
// found in its _meta section.
func (d *Duck) LoadDuckfile(ctx context.Context, duckfile url.URL, recurse bool) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before execution: %w", err)
	}

	if _, exists := d.Duckfiles[duckfile.String()]; exists {
		return nil
	}

	d.Duckfiles[duckfile.String()] = duckfile

	var k *koanf.Koanf
	var err error

	switch duckfile.Scheme {
	case "file":
		k, err = loadFileURL(ctx, duckfile)
		if err != nil {
			return err
		}
	case "http", "https":
		k, err = loadHTTPURL(ctx, duckfile)
		if err != nil {
			return err
		}
	case "s3", "gs", "azblob":
		k, err = loadCloudURL(ctx, duckfile)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported scheme: %s", duckfile.Scheme)
	}

	// Get all top level keys from the koanf object. does not load _meta key as that's reserved.
	for _, key := range k.MapKeys("") {
		// don't load _meta key, if recurse is true, load up the list of files inside it and start baking targets.
		if key == "_meta" {
			if recurse {
				if deps := k.Strings("_meta" + ModifiedColon + "dependencies"); len(deps) > 0 {
					for _, dep := range deps {
						depURLs, err := GetDuckfiles(ctx, dep)
						if err != nil {
							return fmt.Errorf("failed to extract duckfile urls for dependency %s: %w", dep, err)
						}
						for _, depURL := range depURLs {
							if err := d.LoadDuckfile(ctx, depURL, false); err != nil {
								return fmt.Errorf("failed to load dependency duckfile %s: %w", depURL.String(), err)
							}
						}
					}
				}
			}
			continue
		}

		// Get the configuration for this target
		targetConfig := k.Cut(key)
		if err := d.appendTarget(ctx, key, targetConfig); err != nil {
			return fmt.Errorf("failed to append target %s: %w", key, err)
		}
	}

	return nil
}

func loadFileURL(ctx context.Context, duckfile url.URL) (*koanf.Koanf, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	k := koanf.New(ModifiedColon)
	err := k.Load(file.Provider(duckfile.Path), yaml.Parser())
	if err != nil {
		return nil, fmt.Errorf("failed to load file from %s: %w", duckfile.Path, err)
	}
	return k, nil
}

func loadHTTPURL(ctx context.Context, duckfile url.URL) (*koanf.Koanf, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	k := koanf.New(ModifiedColon)
	resp, err := http.Get(duckfile.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", duckfile.String(), err)
	}
	defer resp.Body.Close()

	// Read all contents into memory
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", duckfile.String(), err)
	}

	// Load the byte slice into koanf
	err = k.Load(rawbytes.Provider(data), yaml.Parser())
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml from %s: %w", duckfile.String(), err)
	}
	return k, nil
}

func loadCloudURL(ctx context.Context, duckfile url.URL) (*koanf.Koanf, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before execution: %w", err)
	}

	k := koanf.New(ModifiedColon)
	bucketURL := fmt.Sprintf("%s://%s%s", duckfile.Scheme, duckfile.Host, duckfile.Path)
	if duckfile.RawQuery != "" {
		bucketURL = fmt.Sprintf("%s?%s", bucketURL, duckfile.RawQuery)
	}

	bucket, err := blob.OpenBucket(ctx, bucketURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open bucket %s: %w", bucketURL, err)
	}
	defer bucket.Close()

	// Create a reader for the blob
	key := strings.TrimPrefix(duckfile.Path, "/")
	reader, err := bucket.NewReader(ctx, key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader for %s: %w", key, err)
	}
	defer reader.Close()

	// Read all contents into memory
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read contents from %s: %w", key, err)
	}

	// Load the byte slice into koanf
	err = k.Load(rawbytes.Provider(data), yaml.Parser())
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml from %s: %w", duckfile.String(), err)
	}
	return k, nil
}

// GetDuckfiles takes a string and returns a list of urls.
func GetDuckfiles(ctx context.Context, floc string) ([]url.URL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var u *url.URL
	var err error

	// Check if it looks like a URL first by checking for scheme://
	if strings.Contains(floc, "://") {
		u, err = url.Parse(floc)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %s: %w", floc, err)
		}
	} else {
		// Not a URL, construct a file:// URL
		absPath, err := filepath.Abs(floc)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %w", floc, err)
		}
		u = &url.URL{
			Scheme: "file",
			Path:   absPath,
		}
		slog.Debug("Converting floc to file:// url", "floc", floc, "url", u.String())
	}
	slog.Debug("Extracting duckfiles from url", "url", u.String())

	switch u.Scheme {
	case "file":
		return handleFileURL(ctx, u)
	case "s3", "gs", "azblob":
		return handleCloudURL(ctx, u, floc)
	case "http", "https":
		return handleHTTPURL(ctx, u, floc)
	default:
		return nil, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
}

func handleFileURL(ctx context.Context, u *url.URL) ([]url.URL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var extracted []url.URL

	fileInfo, err := os.Stat(u.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", u.Path, err)
	}

	if fileInfo.IsDir() {
		// Get list of files in directory
		files, err := os.ReadDir(u.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", u.Path, err)
		}
		for _, file := range files {
			filename := file.Name()
			if !file.IsDir() && isDuckfile(filename) {
				fileURL := url.URL{
					Scheme: "file",
					Path:   filepath.Join(u.Path, filename),
				}
				extracted = append(extracted, fileURL)
			}
		}
	} else {
		extracted = append(extracted, *u)
	}

	return extracted, nil
}

func handleCloudURL(ctx context.Context, u *url.URL, floc string) ([]url.URL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var extracted []url.URL

	bucketURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	if u.RawQuery != "" {
		bucketURL = fmt.Sprintf("%s?%s", bucketURL, u.RawQuery)
	}
	bucket, err := blob.OpenBucket(ctx, bucketURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open bucket %s: %w", bucketURL, err)
	}
	defer bucket.Close()

	prefix := strings.TrimPrefix(u.Path, "/")
	if strings.HasSuffix(prefix, "/") || prefix == "" {
		// List objects in bucket/prefix
		iter := bucket.List(&blob.ListOptions{Prefix: prefix})
		for {
			obj, err := iter.Next(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("failed to list objects in %s: %w", bucketURL, err)
			}
			filename := obj.Key
			if isDuckfile(filename) {
				cloudURL := url.URL{
					Scheme:   u.Scheme,
					Host:     u.Host,
					Path:     "/" + filename,
					RawQuery: u.RawQuery,
				}
				extracted = append(extracted, cloudURL)
			}
		}
	} else {
		// Single object
		// Check if object exists
		exists, err := bucket.Exists(ctx, prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to check existence of %s in %s: %w", prefix, bucketURL, err)
		}
		if exists {
			parsedURL, err := url.Parse(floc)
			if err != nil {
				return nil, fmt.Errorf("failed to parse URL %s: %w", floc, err)
			}
			extracted = append(extracted, *parsedURL)
		}
	}

	return extracted, nil
}

func handleHTTPURL(ctx context.Context, u *url.URL, _ string) ([]url.URL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return []url.URL{*u}, nil
}

// isDuckfile checks if a filename matches any of the valid Duckfile patterns
func isDuckfile(filename string) bool {
	return strings.HasSuffix(filename, ".Duckfile") ||
		strings.HasSuffix(filename, ".duck") ||
		strings.HasSuffix(filename, ".duckfile") ||
		strings.HasSuffix(filename, ".duck.yaml") ||
		strings.HasSuffix(filename, ".duck.yml") ||
		filename == "Duckfile"
}
