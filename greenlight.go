package greenlight

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var Version = ""

type Greenlight struct {
	Config *Config

	state           *State
	startUpChecks   []Checker
	readinessChecks []Checker
	responder       *Responder
	ch              chan Signal
}

func Run(ctx context.Context, cli *CLI) error {
	if cli.Version {
		fmt.Println(Version)
		return nil
	}

	if cli.Debug {
		logLevel.Set(slog.LevelDebug)
	}
	slog.Info(fmt.Sprintf("starting greenlight version %s", Version))
	cfg, err := LoadConfig(ctx, cli.Config)
	if err != nil {
		return err
	}
	slog.Info("config loaded", slog.String("config", cli.Config))
	slog.Debug("config", "parsed", cfg)
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
		checker, err := NewChecker(c)
		if err != nil {
			return nil, err
		}
		g.startUpChecks = append(g.startUpChecks, checker)
	}
	for _, c := range cfg.Readiness.Checks {
		checker, err := NewChecker(c)
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
	logger := slog.With("phase", g.state.Phase)
	logger.Info("start phase")
	if t := g.Config.StartUp.GracePeriod; t > 0 {
		logger.Info(fmt.Sprintf("sleeping grace period %s", t))
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
			logger.Info("checks failed",
				slog.Int("index", int(g.state.CheckIndex)),
				slog.String("error", err.Error()))
			logger.Info(fmt.Sprintf("sleeping %s", g.Config.StartUp.Interval))
			time.Sleep(g.Config.StartUp.Interval)
		} else {
			g.state.NextPhase()
			logger.Info("all checks succeeded! go to next phase")
			return nil
		}
	}
}

func (g *Greenlight) CheckStartUp(ctx context.Context) error {
	ctx = context.WithValue(ctx, stateKey, g.state)
	for i := g.state.CheckIndex; i < numofCheckers(len(g.startUpChecks)); i++ {
		g.state.CheckIndex = numofCheckers(i)
		check := g.startUpChecks[i]
		now := time.Now()
		if err := check.Run(ctx); err != nil {
			return fmt.Errorf("check index:%d name:%s failed: %w", i, check.Name(), err)
		}
		elapsed := time.Since(now)
		slog.Info("check succeeded",
			slog.Int("index", int(i)), slog.String("name", check.Name()),
			slog.String("elapsed", elapsed.String()),
		)
	}
	return nil
}

func (g *Greenlight) RunRedinessChecks(ctx context.Context) error {
	logger := slog.With("phase", g.state.Phase)
	logger.Info("start phase")
	if t := g.Config.Readiness.GracePeriod; t > 0 {
		logger.Info(fmt.Sprintf("sleeping grace period %s", t))
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
			logger.Warn("some checks failed", slog.String("error", err.Error()))
			g.Send(SignalYellow)
		} else {
			logger.Debug("all checks succeeded!")
			g.Send(SignalGreen)
		}
		time.Sleep(g.Config.Readiness.Interval)
	}
}

func (g *Greenlight) CheckRediness(ctx context.Context) error {
	ctx = context.WithValue(ctx, stateKey, g.state)
	logger := slog.With("phase", g.state.Phase)

	var errs error
	// rediness checks allways run all.
	for i, check := range g.readinessChecks {
		g.state.CheckIndex = numofCheckers(i)
		now := time.Now()
		if err := check.Run(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("check %d failed: %w", i, err))
		}
		elapsed := time.Since(now)
		logger.Debug("check succeeded",
			slog.Int("index", int(i)), slog.String("name", check.Name()),
			slog.String("elapsed", elapsed.String()),
		)
	}
	return errs
}
