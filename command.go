package greenlight

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/Songmu/wrapcommander"
	"github.com/mattn/go-shellwords"
)

type CommandCheckConfig struct {
	Run string `yaml:"run"`
}

type CommandChecker struct {
	name     string
	commands []string
	timeout  time.Duration
}

func NewCommandChecker(cfg *CheckConfig) (*CommandChecker, error) {
	cmds, err := shellwords.Parse(cfg.Command.Run)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %s %w", cfg.Command.Run, err)
	}
	return &CommandChecker{
		name:     cfg.Name,
		commands: cmds,
		timeout:  cfg.Timeout,
	}, nil
}

func (c *CommandChecker) Name() string {
	return c.name
}

func (c *CommandChecker) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	logger := newLoggerFromContext(ctx).With(
		"name", c.name,
		"module", "commandchecker",
		"commands", fmt.Sprintf("%v", c.commands),
	)
	logger.Debug("executing command")
	var cmd *exec.Cmd
	switch len(c.commands) {
	case 0:
		return errors.New("no command")
	case 1:
		cmd = exec.CommandContext(ctx, c.commands[0])
	default:
		cmd = exec.CommandContext(ctx, c.commands[0], c.commands[1:]...)
	}
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 3 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Info("command failed",
			slog.Int("exit_code", wrapcommander.ResolveExitCode(err)),
			slog.String("output", string(out)),
			slog.String("error", err.Error()),
		)
		return err
	}
	logger.Debug("command succeeded",
		slog.Int("exit_code", wrapcommander.ResolveExitCode(err)),
		slog.String("output", string(out)),
	)
	return nil
}
