package greenlight

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

type Responder struct {
	addr    string
	current Signal
	mu      *sync.Mutex
	ch      chan Signal
	logger  *slog.Logger
}

func NewResponder(cfg *ResponderConfig) (*Responder, chan Signal) {
	ch := make(chan Signal, 1)
	return &Responder{
		addr:   cfg.Addr,
		mu:     &sync.Mutex{},
		ch:     ch,
		logger: slog.With("module", "responder"),
	}, ch
}

func (r *Responder) Run(ctx context.Context) error {
	r.logger.Info("starting responder")
	defer r.logger.Info("responder exited")
	srv := http.Server{
		Addr:    r.addr,
		Handler: r.handler(),
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown(ctx)
	}()
	go r.signalLisetener(ctx)

	r.logger.Info(fmt.Sprintf("listening on %s", r.addr))
	if err := srv.ListenAndServe(); err != nil {
		r.logger.Error("failed to listen and serve", slog.String("error", err.Error()))
		return err
	}
	return nil
}

func (r *Responder) setCurrentSignal(s Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == s {
		return
	} else {
		r.logger.Info(fmt.Sprintf("signal changed %s -> %s", r.current, s))
	}
	r.current = s
}

func (r *Responder) getCurrentSignal() Signal {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.current
}

func (r *Responder) signalLisetener(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case s := <-r.ch:
			r.logger.Debug("received", slog.String("signal", string(s)))
			r.setCurrentSignal(s)
		}
	}
}

func (r *Responder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		code := http.StatusOK
		msg := "OK"

		defer func() {
			req.Body.Close()
			r.logger.Info(
				msg,
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.String("proto", req.Proto),
				slog.Int("status", code),
				slog.String("remote_addr", req.RemoteAddr),
				slog.String("host", req.Host),
				slog.String("user_agent", req.UserAgent()),
			)
		}()

		w.Header().Set("Content-Type", "text/plain")
		s := r.getCurrentSignal()
		switch s {
		case SignalGreen:
		case SignalYellow, SignalRed:
			code = http.StatusServiceUnavailable
			msg = "Service Unavailable"
		default:
			r.logger.Warn(fmt.Sprintf("unknown signal: %s", s))
			code = http.StatusInternalServerError
			msg = "Internal Server Error"
		}
		w.WriteHeader(code)
		fmt.Fprintln(w, msg)
	})
}
