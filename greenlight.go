package greenlight

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/Songmu/wrapcommander"
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
	defer func() {
		slog.Info("greenlight exited")
	}()

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
	case <-redinessErr:
		break // rediness never returns error.
	case err := <-responderErr:
		if err != nil {
			return err
		}
	case err := <-childCommandErr:
		if err != nil {
			return err
		}
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
	logger.Info("starting checks for startup")
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
				slog.String("name", g.startUpChecks[g.state.CheckIndex].Name()),
				slog.String("error", err.Error()))
			logger.Info(fmt.Sprintf("sleeping %s", g.Config.StartUp.Interval))
			time.Sleep(g.Config.StartUp.Interval)
			continue
		}
		g.state.NextPhase()
		logger.Info("all checks succeeded! go to next phase")
		ch <- nil
		return
	}
}

func (g *Greenlight) CheckStartUp(ctx context.Context) error {
	ctx = context.WithValue(ctx, stateKey, g.state)
	for i := g.state.CheckIndex; i < numofCheckers(len(g.startUpChecks)); i++ {
		g.state.CheckIndex = numofCheckers(i)
		check := g.startUpChecks[i]
		now := time.Now()
		if err := check.Run(ctx); err != nil {
			return err
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
	logger.Info("starting checks for readiness")
	if t := g.Config.Readiness.GracePeriod; t > 0 {
		logger.Info(fmt.Sprintf("sleeping grace period %s", t))
		time.Sleep(t)
	}
	for {
		err := g.CheckRediness(ctx)
		if err != nil {
			logger.Warn("some checks failed", slog.String("error", err.Error()))
			g.Send(SignalYellow)
		} else {
			logger.Debug("all checks succeeded!")
			g.Send(SignalGreen)
		}
		select {
		case <-time.After(g.Config.Readiness.Interval):
		case <-ctx.Done():
			ch <- nil
			return
		}
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
	logger := newLoggerFromContext(ctx).With(
		"module", "childcommand",
		"commands", fmt.Sprintf("%v", commands),
	)

	var ignoreExitError bool
	logger.Info("starting child command")
	cmd := exec.CommandContext(ctx, commands[0], commands[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Cancel = func() error {
		logger.Info("sending SIGTERM to child command")
		ignoreExitError = true
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 30 * time.Second // TODO: configurable

	err := cmd.Run()
	if err != nil && !ignoreExitError {
		exitCode := wrapcommander.ResolveExitCode(err)
		logger.Error("child command failed",
			slog.String("error", err.Error()),
			slog.Int("exit_code", exitCode),
		)
		ch <- err
		return
	}
	ch <- fmt.Errorf("child command exited: %s", cmd.ProcessState.String())
}
