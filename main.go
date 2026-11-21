package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"boardgamesort/internal/config"
	"boardgamesort/internal/database"
	"boardgamesort/internal/httpserver"
	"boardgamesort/internal/logging"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	appConfig, err := config.Load(workingDirectory)
	if err != nil {
		return fmt.Errorf("Config Load %w", err)
	}

	databaseConnection, err := database.Open(ctx, appConfig.DatabaseURL)
	if err != nil {
		return err
	}
	defer databaseConnection.Close()
	appLogger := logging.New()

	app, err := httpserver.New(databaseConnection, appLogger, httpserver.Options{
		AppOrigin:         appConfig.AppOrigin,
		TemplateDirectory: filepath.Join(workingDirectory, "web", "templates"),
		PublicDirectory:   filepath.Join(workingDirectory, "web", "public"),
	})
	if err != nil {
		return err
	}
	defer app.Close()
	applicationServer := httpserver.NewServer(fmt.Sprintf(":%d", appConfig.Port), app.Handler)
	serverErrors := make(chan error, 1)
	serve(applicationServer, fmt.Sprintf("Boardgamesort is running at http://localhost:%d", appConfig.Port), serverErrors)
	select {
	case <-ctx.Done():
		return shutdownServer(applicationServer)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP server: %w", err)
	}
}

func serve(server *http.Server, message string, serverErrors chan<- error) {
	go func() {
		fmt.Println(message)
		serverErrors <- server.ListenAndServe()
	}()
}

func shutdownServer(server *http.Server) error {
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownContext)
}
