package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/vishn007/go-service-template/app/services/user-service/handlers"
	"github.com/vishn007/go-service-template/foundation/logger"
	"go.uber.org/zap"
)

var build = "develop"

func main() {
	log, err := logger.New("USER-SERVICE")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer log.Sync()

	if err := run(log); err != nil {
		log.Errorw("startup", "ERROR", err)
		log.Sync()
		os.Exit(1)
	}
}

func run(log *logger.Logger) error {
	log.Infow("startup", "GOMAXPROCS", runtime.GOMAXPROCS(0), "BUILD - ", build)

	//----------------------Service Start-------------------------//
	log.Infow("starting service", "version", build)
	defer log.Infow("shutdown complete")

	log.Infow("startup", "status", "initializing V1 API support")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	apiMux := handlers.APIMux(handlers.APIMuxConfig{
		Shutdown: shutdown,
		Log:      log,
	})

	api := http.Server{
		Addr:         "0.0.0.0:3000",
		Handler:      apiMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  2 * time.Minute,
		ErrorLog:     zap.NewStdLog(log.Desugar()),
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Infow("startup", "status", "api router started", "host", api.Addr)
		serverErrors <- api.ListenAndServe()
	}()

	// -------------------------------------------------------------------------
	// Shutdown

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Infow("shutdown", "status", "shutdown started", "signal", sig)
		defer log.Infow("shutdown", "status", "shutdown complete", "signal", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if err := api.Shutdown(ctx); err != nil {
			api.Close()
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil

}
