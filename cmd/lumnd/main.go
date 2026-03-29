package main

import (
	"log"
	"os"

	"github.com/pretodev/lumn/internal/daemon"
)

func main() {
	paths, err := daemon.DefaultPaths()
	if err != nil {
		log.Fatal(err)
	}

	cfg := daemon.LoadConfig(paths, os.Stderr, func(format string, args ...any) {
		log.Printf(format, args...)
	})

	server, err := daemon.New(cfg, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	if err := server.Run(); err != nil {
		log.Fatal(err)
	}

	signals := make(chan os.Signal, 1)
	notifySignals(signals)
	for range signals {
		if err := server.Shutdown(cfg.ShutdownTimeout); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			continue
		}
		return
	}
}
