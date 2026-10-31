package main

import (
	"context"
	"github.com/cuihairu/croupier/internal/analytics/worker"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	w, err := worker.NewWorker()
	if err != nil {
		slog.Error("init worker", "err", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		if err := w.Run(ctx); err != nil {
			slog.Error("run", "err", err)
		}
	}()
	slog.Info("analytics-worker started")
	<-ctx.Done()
	// Run 内部已按 DATA-015 顺序停子协程并做带截止最终 flush；main 只需
	// 等待 Run 完整返回，不再用固定 200ms 猜测刷新何时结束。
	<-runDone
}
