package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ollevche/marko/internal/domain/transcript"
	"github.com/ollevche/marko/internal/restapi"
	storage "github.com/ollevche/marko/internal/storage/obsidian"
	"github.com/ollevche/marko/pkg/obsidian"
)

func main() {
	if err := runREST(); err != nil {
		log.Fatalf("Failed to run REST API: %v", err)
	}
}

func runREST() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	userLocation, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		return fmt.Errorf("loading user location: %w", err)
	}

	bluedotSecret := os.Getenv("BLUEDOT_WEBHOOK_SECRET")
	if bluedotSecret == "" {
		return errors.New("no bluedot secret found")
	}

	obsidianCreds := strings.SplitN(os.Getenv("OBSIDIAN_CREDS"), ":", 2)
	if len(obsidianCreds) != 2 {
		return fmt.Errorf("invalid obsidian creds secret")
	}

	obsidianConfig := storage.Config{
		Config: obsidian.Config{
			Email:    obsidianCreds[0],
			Password: obsidianCreds[1],
			Vault: obsidian.VaultConfig{
				Path:     os.Getenv("OBSIDIAN_VAULT_PATH"),
				Name:     os.Getenv("OBSIDIAN_VAULT_NAME"),
				Password: os.Getenv("OBSIDIAN_VAULT_PASSWORD"),
			},
		},
		TranscriptsPathPrefix: os.Getenv("OBSIDIAN_TRANSCRIPTS_PATH_PREFIX"),
	}

	storage, err := storage.NewStorage(ctx, obsidianConfig)
	if err != nil {
		return fmt.Errorf("creating storage: %w", err)
	}
	defer storage.Close(ctx)

	server, err := restapi.BuildServer(restapi.Config{
		BluedotSecret: bluedotSecret,
		Addr:          addr(),
	}, &transcript.Service{
		UserLocation: userLocation,
		Storage:      storage,
	})
	if err != nil {
		return fmt.Errorf("building server: %w", err)
	}

	serveErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	log.Printf("listening on %s...", server.Addr)

	select {
	case err := <-serveErr:
		return fmt.Errorf("server stopped: %w", err)
	case <-ctx.Done():
	}

	log.Print("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

func addr() string {
	p := os.Getenv("PORT")
	if p == "" {
		p = "8080"
	}
	return ":" + p
}
