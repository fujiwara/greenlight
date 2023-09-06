package greenlight

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/goccy/go-yaml"
)

const (
	DefaultCheckInterval = 6 * time.Second
	DefaultCheckTimeout  = 5 * time.Second
	DefaultListenAddr    = ":8080"
)

type Config struct {
	Responder *ResponderConfig `yaml:"responder"`
	StartUp   *PhaseConfig     `yaml:"startup"`
	Readiness *PhaseConfig     `yaml:"readiness"`
}

type ResponderConfig struct {
	Addr string `yaml:"addr"`
}

type PhaseConfig struct {
	Checks      []*CheckConfig `yaml:"checks"`
	Interval    time.Duration  `yaml:"interval"`
	GracePeriod time.Duration  `yaml:"grace_period"`
}

type CheckConfig struct {
	Name    string        `yaml:"name"`
	Timeout time.Duration `yaml:"timeout"`

	Command *CommandCheckConfig `yaml:"command"`
	TCP     *TCPCheckConfig     `yaml:"tcp"`
	HTTP    *HTTPCheckConfig    `yaml:"http"`
}

func LoadConfig(ctx context.Context, src string) (*Config, error) {
	config := &Config{
		StartUp: &PhaseConfig{
			Interval: DefaultCheckInterval,
		},
		Readiness: &PhaseConfig{
			Interval: DefaultCheckInterval,
		},
		Responder: &ResponderConfig{
			Addr: DefaultListenAddr,
		},
	}
	b, err := loadURL(ctx, src)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(b, config); err != nil {
		return nil, err
	}
	for _, c := range config.StartUp.Checks {
		if c.Timeout == 0 {
			c.Timeout = DefaultCheckTimeout
		}
	}
	for _, c := range config.Readiness.Checks {
		if c.Timeout == 0 {
			c.Timeout = DefaultCheckTimeout
		}
	}
	return config, nil
}

func loadURL(ctx context.Context, s string) ([]byte, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid url %s: %w", s, err)
	}
	switch u.Scheme {
	case "http", "https":
		return loadHTTP(ctx, u)
	case "file", "": // empty scheme is treated as file
		return os.ReadFile(u.Path)
	case "s3":
		return loadS3(ctx, u)
	default:
		return nil, fmt.Errorf("invalid url %s: scheme must be http, https, file, or s3", s)
	}
}

func loadHTTP(ctx context.Context, u *url.URL) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func loadS3(ctx context.Context, u *url.URL) ([]byte, error) {
	awscfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	svc := s3.NewFromConfig(awscfg)
	bucket, key := u.Host, strings.TrimPrefix(u.Path, "/")
	out, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}
