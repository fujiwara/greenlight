package greenlight

import (
	"context"
	"log/slog"
	"os"
)

var logLevel = new(slog.LevelVar)
var logger *slog.Logger

func init() {
	opts := slog.HandlerOptions{
		Level: logLevel,
	}
	logger = slog.New(slog.NewJSONHandler(os.Stderr, &opts))
	slog.SetDefault(logger)
}

func newLoggerFromContext(ctx context.Context) *slog.Logger {
	if s := ctx.Value(stateKey); s != nil {
		state := s.(*State)
		return slog.With("phase", state.Phase, "index", state.CheckIndex)
	}
	return logger
}
