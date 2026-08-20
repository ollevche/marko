// Command marko runs an HTTP server that receives Bluedot meeting transcript
// webhooks and prints them to stdout.
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

	svix "github.com/svix/svix-webhooks/go"
)

func main() {
	if err := listenAndServe(); err != nil {
		log.Fatalf("Listening and serving: %v", err)
	}
}

func listenAndServe() error {
	bluedotSecret := os.Getenv("BLUEDOT_WEBHOOK_SECRET")
	if bluedotSecret == "" {
		return errors.New("no bluedot secret found")
	}

	bluedotVerifier, err := svix.NewWebhook(bluedotSecret)
	if err != nil {
		return fmt.Errorf("initing bluedot webhook verifier: %w", err)
	}

	obsidianCreds := strings.SplitN(os.Getenv("OBSIDIAN_CREDS"), ":", 2)
	if len(obsidianCreds) != 3 {
		return fmt.Errorf("invalid obsidian creds secret")
	}

	obsidianConfig := obsidianConfig{
		Email:                 obsidianCreds[0],
		Password:              obsidianCreds[1],
		VaultPath:             os.Getenv("OBSIDIAN_VAULT_PATH"),
		TranscriptsPathPrefix: os.Getenv("OBSIDIAN_TRANSCRIPTS_PATH_PREFIX"),
	}

	if obsidianConfig.VaultPath == "" || obsidianConfig.TranscriptsPathPrefix == "" {
		return fmt.Errorf("missing configuration for obsidian")
	}

	obsidian, err := newObsidian(obsidianConfig)
	if err != nil {
		return fmt.Errorf("creating obsidian: %w", err)
	}
	defer obsidian.close()

	mux := http.NewServeMux()

	mux.Handle("POST /meetings/transcripts", &transcriptHandler{
		verifier: bluedotVerifier,
		obsidian: obsidian,
	})

	srv := &http.Server{
		Addr:              addr(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	log.Printf("listening on %s...", srv.Addr)

	select {
	case err := <-serveErr:
		return fmt.Errorf("server stopped: %w", err)
	case <-ctx.Done():
	}

	log.Print("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

func addr() string {
	p := os.Getenv("PORT")
	if p == "" {
		p = "8080"
	}
	return ":" + p
}
