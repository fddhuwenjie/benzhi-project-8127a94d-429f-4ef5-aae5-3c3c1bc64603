package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"oralhistory/internal/application"
	"oralhistory/internal/persistence"
	"oralhistory/internal/webui"
)

type runtime struct {
	server   *http.Server
	listener net.Listener
	app      *application.Service
}

func assemble(cfg config) (*runtime, error) {
	store, err := persistence.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("打开持久层失败: %w", err)
	}
	app := application.NewService(store)
	web := webui.NewServer(app)
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("监听 %s 失败: %w", cfg.Addr, err)
	}
	server := &http.Server{
		Handler:           securityHeaders(web.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return &runtime{server: server, listener: listener, app: app}, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (rt *runtime) serve() <-chan error {
	result := make(chan error, 1)
	go func() {
		err := rt.server.Serve(rt.listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		result <- err
	}()
	return result
}

func (rt *runtime) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return rt.server.Shutdown(ctx)
}

func runServer(cfg config) error {
	rt, err := assemble(cfg)
	if err != nil {
		return err
	}
	diagnostic, _ := json.Marshal(map[string]any{"level": "info", "message": "服务已启动", "addr": rt.listener.Addr().String(), "data_dir": cfg.DataDir})
	log.Print(string(diagnostic))
	serveResult := rt.serve()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err := <-serveResult:
		return err
	case sig := <-signals:
		log.Printf("收到终止信号 %s，开始有界关闭", sig)
		if err := rt.shutdown(); err != nil {
			return err
		}
		return <-serveResult
	}
}
