package greenlight

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type Responder struct {
	addr    string
	current Signal
	mu      *sync.Mutex
	ch      chan Signal
}

func NewResponder(cfg *ResponderConfig) (*Responder, chan Signal) {
	ch := make(chan Signal, 1)
	return &Responder{
		addr: cfg.Addr,
		mu:   &sync.Mutex{},
		ch:   ch,
	}, ch
}

func (r *Responder) Run(ctx context.Context) error {
	srv := http.Server{
		Addr:    r.addr,
		Handler: r.handler(),
	}
	go func() {
		<-ctx.Done()
		srv.Shutdown(ctx)
	}()
	go r.signalLisetener(ctx)

	log.Printf("[info] [responder] listening on %s", r.addr)
	return srv.ListenAndServe()
}

func (r *Responder) setCurrentSignal(s Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current == s {
		return
	} else {
		log.Printf("[info] [responder] signal changed %s -> %s", r.current, s)
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
			log.Printf("[debug] [responder] signal received: %s", s)
			r.setCurrentSignal(s)
		}
	}
}

func (r *Responder) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		code := http.StatusOK
		msg := "OK"
		s := r.getCurrentSignal()
		switch s {
		case SignalGreen:
		case SignalYellow, SignalRed:
			code = http.StatusServiceUnavailable
			msg = "Service Unavailable"
		default:
			log.Printf("[warn] [responder] unknown signal: %s", s)
			code = http.StatusInternalServerError
			msg = "Internal Server Error"
		}
		w.WriteHeader(code)
		fmt.Fprintln(w, msg)
	})
}
