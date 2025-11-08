package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"
    "github.com/cuihairu/croupier/internal/analytics/worker"
)

func main() {
    w, err := worker.NewWorker()
    if err != nil { slog.Error("init worker", "err", err); os.Exit(1) }
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()
    go func(){ if err := w.Run(ctx); err != nil { slog.Error("run", "err", err) } }()
    slog.Info("analytics-worker started")
    <-ctx.Done()
    time.Sleep(200 * time.Millisecond)
}

