package greenlight

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Greenlight struct {
	Config *Config

	phase           phase
	startUpChecks   []Checker
	readinessChecks []Checker
}

func Run(ctx context.Context) error {
	fmt.Println("greenlight!")
	cfg, err := LoadConfig(ctx, "greenlight.yaml")
	if err != nil {
		return err
	}
	g, err := NewGreenlight(cfg)
	if err != nil {
		return err
	}
	return g.Run(ctx)
}

func NewGreenlight(cfg *Config) (*Greenlight, error) {
	g := &Greenlight{
		Config: cfg,
		phase:  phaseStartUp,
	}
	for _, c := range cfg.StartUp.Checks {
		checker, err := NewCommandChecker(&c)
		if err != nil {
			return nil, err
		}
		g.startUpChecks = append(g.startUpChecks, checker)
	}
	for _, c := range cfg.Readiness.Checks {
		checker, err := NewCommandChecker(&c)
		if err != nil {
			return nil, err
		}
		g.readinessChecks = append(g.readinessChecks, checker)
	}
	return g, nil
}

func (g *Greenlight) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		switch g.phase {
		case phaseStartUp:
			err := g.CheckStartUp(context.WithValue(ctx, phaseKey, g.phase))
			if err != nil {
				log.Printf("[info] [phase %s] checks failed: %s", g.phase, err)
				log.Printf("[info] [phase %s] sleep %s", g.phase, g.Config.StartUp.Interval)
				time.Sleep(g.Config.StartUp.Interval)
			} else {
				log.Printf("[info] [phase %s] all checks succeeded! moving to running phase", g.phase)
				g.phase = phaseRunning
			}
		case phaseRunning:
			log.Println("[info] TODO running checks. Goodbye!")
			return nil
		}
	}
}

func (g *Greenlight) CheckStartUp(ctx context.Context) error {
	for i, checker := range g.startUpChecks {
		err := checker.Run(context.WithValue(ctx, numofCheckersKey, numofCheckers(i)))
		if err != nil {
			return err
		}
	}
	return nil
}
