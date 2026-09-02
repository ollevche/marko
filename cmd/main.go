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

	"github.com/ollevche/marko/internal/domain/notification"
	"github.com/ollevche/marko/internal/domain/transcript"
	"github.com/ollevche/marko/internal/githubsource"
	"github.com/ollevche/marko/internal/obsidianstore"
	"github.com/ollevche/marko/internal/restapi"
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

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		return errors.New("no github token found")
	}

	github, err := githubsource.New(githubsource.Config{
		Token: githubToken,
	})
	if err != nil {
		return fmt.Errorf("initing github source: %w", err)
	}

	obsidianCreds := strings.SplitN(os.Getenv("OBSIDIAN_CREDS"), ":", 2)
	if len(obsidianCreds) != 2 {
		return fmt.Errorf("invalid obsidian creds secret")
	}

	storeConfig := obsidianstore.Config{
		Config: obsidian.Config{
			Email:    obsidianCreds[0],
			Password: obsidianCreds[1],
			Vault: obsidian.VaultConfig{
				Path:     os.Getenv("OBSIDIAN_VAULT_PATH"),
				Name:     os.Getenv("OBSIDIAN_VAULT_NAME"),
				Password: os.Getenv("OBSIDIAN_VAULT_PASSWORD"),
			},
		},
		TranscriptsPathPrefix:   os.Getenv("OBSIDIAN_TRANSCRIPTS_PATH_PREFIX"),
		NotificationsPathPrefix: os.Getenv("OBSIDIAN_NOTIFICATIONS_PATH_PREFIX"),
	}

	store, err := obsidianstore.New(ctx, storeConfig)
	if err != nil {
		return fmt.Errorf("creating store: %w", err)
	}
	// SIGTERM cancels ctx before this runs, and exec.CommandContext will not start
	// a process on a cancelled context, so give the close its own deadline.
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()

		if err := store.Close(closeCtx); err != nil {
			log.Printf("Failed to close store: %v", err)
		}
	}()

	server, err := restapi.BuildServer(restapi.Config{
		BluedotSecret: bluedotSecret,
		Addr:          addr(),
	}, restapi.Services{
		Transcripts: &transcript.Service{
			UserLocation: userLocation,
			Store:        store,
		},
		Notifications: &notification.Service{
			Store:           store,
			Source:          github,
			MinListingDelay: 0,
		},
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
