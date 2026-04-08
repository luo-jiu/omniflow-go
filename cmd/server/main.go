package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"omniflow-go/internal/bootstrap"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, cleanup, err := bootstrap.InitializeApplication(*configPath)
	if err != nil {
		log.Fatalf("initialize application: %v", err)
	}
	defer cleanup()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
