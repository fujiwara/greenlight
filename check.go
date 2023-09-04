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
	Run(ctx context.Context) error
}

type CommandChecker struct {
	Commands []string
	Timeout  time.Duration
}

func NewCommandChecker(cfg *CheckConfig) (*CommandChecker, error) {
	cmds, err := shellwords.Parse(cfg.Command)
	if err != nil {
		return nil, err
	}
	return &CommandChecker{
		Commands: cmds,
		Timeout:  cfg.Timeout,
	}, nil
}

func (c *CommandChecker) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	p := ctx.Value(phaseKey).(phase)
	n := ctx.Value(numofCheckersKey).(numofCheckers)

	log.Printf("[info] [phase %s] [%d] running command: %s", p, n, c.Commands)
	var cmd *exec.Cmd
	switch len(c.Commands) {
	case 0:
		return errors.New("no command")
	case 1:
		cmd = exec.CommandContext(ctx, c.Commands[0])
	default:
		cmd = exec.CommandContext(ctx, c.Commands[0], c.Commands[1:]...)
	}
	cmd.Env = append(cmd.Env, os.Environ()...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[info] [phase %s] [%d] %s command failed: %s", p, n, out, err)
		return err
	}
	log.Printf("[info] [phase %s] [%d] command succeeded: %s", p, n, string(out))
	return nil
}
