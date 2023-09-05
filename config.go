package greenlight

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	DefaultCheckInterval = 5 * time.Second
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
	Checks      []CheckConfig `yaml:"checks"`
	Interval    time.Duration `yaml:"interval"`
	GracePeriod time.Duration `yaml:"grace_period"`
}

type CheckConfig struct {
	Name    string        `yaml:"name"`
	Command string        `yaml:"command"`
	Timeout time.Duration `yaml:"timeout"`
}

func LoadConfig(ctx context.Context, path string) (*Config, error) {
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
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(b, config)
	if err != nil {
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