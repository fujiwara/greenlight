package greenlight

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/mattn/go-shellwords"
)

type Checker interface {
	Name() string
	Run(ctx context.Context) error
}

type CommandChecker struct {
	name     string
	commands []string
	timeout  time.Duration
}

func NewCommandChecker(cfg *CheckConfig) (*CommandChecker, error) {
	cmds, err := shellwords.Parse(cfg.Command)
	if err != nil {
		return nil, err
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
	state := ctx.Value(stateKey).(*State)
	p, n := state.Phase, state.CheckIndex

	log.Printf("[info] [phase %s] [index %d] running command: %s", p, n, c.commands)
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[info] [phase %s] [index %d] [name %s] %s command failed: %s", p, n, c.name, out, err)
		return err
	}
	log.Printf("[info] [phase %s] [index %d] [name %s] command succeeded: %s", p, n, c.name, string(out))
	return nil
}
