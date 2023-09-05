package greenlight

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type Greenlight struct {
	Config *Config

	state           *State
	startUpChecks   []Checker
	readinessChecks []Checker
	responder       *Responder
	ch              chan Signal
}

func Run(ctx context.Context, cli *CLI) error {
	cfg, err := LoadConfig(ctx, cli.Config)
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
	responder, ch := NewResponder(cfg.Responder)
	g := &Greenlight{
		Config:    cfg,
		state:     newState(),
		responder: responder,
		ch:        ch,
	}
	for _, c := range cfg.StartUp.Checks {
		checker, err := NewChecker(&c)
		if err != nil {
			return nil, err
		}
		g.startUpChecks = append(g.startUpChecks, checker)
	}
	for _, c := range cfg.Readiness.Checks {
		checker, err := NewChecker(&c)
		if err != nil {
			return nil, err
		}
		g.readinessChecks = append(g.readinessChecks, checker)
	}
	return g, nil
}

func (g *Greenlight) Run(ctx context.Context) error {
	wg := &sync.WaitGroup{}
	if err := g.RunStartUpChecks(ctx); err != nil {
		return err
	}

	// StartUp succeeded. Signal green.
	g.Send(SignalGreen)

	// Run responder.
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		if err := g.responder.Run(ctx); err != nil {
			return
		}
	}(ctx)

	// Run readiness checks.
	if err := g.RunRedinessChecks(ctx); err != nil {
		return err
	}

	wg.Wait()
	return nil
}

func (g *Greenlight) Send(s Signal) {
	g.ch <- s
}

func (g *Greenlight) RunStartUpChecks(ctx context.Context) error {
	log.Printf("[info] [phase %s] start", g.state.Phase)
	if t := g.Config.StartUp.GracePeriod; t > 0 {
		log.Printf("[info] [phase %s] sleeping grace period %s", g.state.Phase, t)
		time.Sleep(t)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		err := g.CheckStartUp(ctx)
		if err != nil {
			log.Printf("[info] [phase %s] [index %d] checks failed: %s", g.state.Phase, g.state.CheckIndex, err)
			log.Printf("[info] [phase %s] sleep %s", g.state.Phase, g.Config.StartUp.Interval)
			time.Sleep(g.Config.StartUp.Interval)
		} else {
			p := g.state.Phase
			g.state.NextPhase()
			log.Printf("[info] [phase %s] all checks succeeded! moving to next phase: %s", p, g.state.Phase)
			return nil
		}
	}
}

func (g *Greenlight) CheckStartUp(ctx context.Context) error {
	ctx = context.WithValue(ctx, stateKey, g.state)
	for i := g.state.CheckIndex; i < numofCheckers(len(g.startUpChecks)); i++ {
		g.state.CheckIndex = numofCheckers(i)
		check := g.startUpChecks[i]
		if err := check.Run(ctx); err != nil {
			return fmt.Errorf("check index:%d name:%s failed: %w", i, check.Name(), err)
		}
	}
	return nil
}

func (g *Greenlight) RunRedinessChecks(ctx context.Context) error {
	log.Printf("[info] [phase %s] start", g.state.Phase)
	if t := g.Config.Readiness.GracePeriod; t > 0 {
		log.Printf("[info] [phase %s] sleeping grace period %s", g.state.Phase, t)
		time.Sleep(t)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		err := g.CheckRediness(ctx)
		if err != nil {
			log.Printf("[info] [phase %s] some checks failed: %s", g.state.Phase, err)
			g.Send(SignalYellow)
		} else {
			log.Printf("[info] [phase %s] all checks succeeded!", g.state.Phase)
			g.Send(SignalGreen)
		}
		time.Sleep(g.Config.Readiness.Interval)
	}
}

func (g *Greenlight) CheckRediness(ctx context.Context) error {
	ctx = context.WithValue(ctx, stateKey, g.state)

	var errs error
	// rediness checks allways run all.
	for i, check := range g.readinessChecks {
		g.state.CheckIndex = numofCheckers(i)
		if err := check.Run(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("check %d failed: %w", i, err))
		}
	}
	return errs
}
