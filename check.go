package greenlight

import (
	"context"
	"fmt"
)

type Checker interface {
	Name() string
	Run(ctx context.Context) error
}

func NewChecker(cfg *CheckConfig) (Checker, error) {
	if cfg.Command != nil {
		return NewCommandChecker(cfg)
	} else if cfg.TCP != nil {
		return NewTCPChecker(cfg)
	} else {
		return nil, fmt.Errorf("invalid check config. command, tcp, or http section is required: %v", cfg)
	}
}
