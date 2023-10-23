package greenlight

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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
	childCmds       []string
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
	g.childCmds = cli.ChildCmds
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

	// Run external command. (optional)
	childCommandErr := make(chan error, 1)
	wg.Add(1)
	go g.RunChildCommand(ctx, wg, childCommandErr)

	// Run startup checks.
	startUpErr := make(chan error, 1)
	wg.Add(1)
	go g.RunStartUpChecks(ctx, wg, startUpErr)

	// Wait for startup checks or child command.
	select {
	case err := <-startUpErr:
		if err != nil {
			return err
		}
	case err := <-childCommandErr:
		return err
	}

	// StartUp succeeded. Signal green.
	g.Send(SignalGreen)

	// Run responder.
	responderErr := make(chan error, 1)
	wg.Add(1)
	go g.RunResponder(ctx, wg, responderErr)

	// Run readiness checks.
	redinessErr := make(chan error, 1)
	wg.Add(1)
	go g.RunRedinessChecks(ctx, wg, redinessErr)

	// Wait for readiness checks or responder or child command.
	select {
	case err := <-redinessErr:
		if err != nil {
			return err
		}
	case err := <-responderErr:
		return err
	case err := <-childCommandErr:
		return err
	}

	wg.Wait()
	return nil
}

func (g *Greenlight) Send(s Signal) {
	g.ch <- s
}

func (g *Greenlight) RunStartUpChecks(ctx context.Context, wg *sync.WaitGroup, ch chan error) {
	defer wg.Done()
	logger := slog.With("phase", g.state.Phase)
	logger.Info("start phase")
	if t := g.Config.StartUp.GracePeriod; t > 0 {
		logger.Info(fmt.Sprintf("sleeping grace period %s", t))
		time.Sleep(t)
	}
	for {
		select {
		case <-ctx.Done():
			ch <- nil
			return
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
			ch <- nil
			return
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

func (g *Greenlight) RunRedinessChecks(ctx context.Context, wg *sync.WaitGroup, ch chan error) {
	defer wg.Done()
	logger := slog.With("phase", g.state.Phase)
	logger.Info("start phase")
	if t := g.Config.Readiness.GracePeriod; t > 0 {
		logger.Info(fmt.Sprintf("sleeping grace period %s", t))
		time.Sleep(t)
	}
	for {
		select {
		case <-ctx.Done():
			ch <- nil
			return
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

func (g *Greenlight) RunResponder(ctx context.Context, wg *sync.WaitGroup, ch chan error) {
	defer wg.Done()
	if err := g.responder.Run(ctx); err != nil {
		ch <- err
	}
}

func (g *Greenlight) RunChildCommand(ctx context.Context, wg *sync.WaitGroup, ch chan error) {
	defer wg.Done()
	commands := g.childCmds
	if len(commands) == 0 {
		return
	}
	// TODO graceful shutdown
	slog.Info("starting child command", slog.String("command", fmt.Sprintf("%v", commands)))
	cmd := exec.CommandContext(ctx, commands[0], commands[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error("child command failed", slog.String("error", err.Error()))
		ch <- err
	}
	ch <- fmt.Errorf("child command exited: %s", cmd.ProcessState.String())
}
