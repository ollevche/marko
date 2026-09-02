package restapi

import (
	"fmt"
	"net/http"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

type Config struct {
	Addr          string
	BluedotSecret string
}

type endpoint interface {
	handleRoutes(func(rule string, handler func(http.ResponseWriter, *http.Request)))
}

type Services struct {
	Transcripts   TranscriptService
	Notifications NotificationService
}

func BuildServer(c Config, s Services) (*http.Server, error) {
	bluedotVerifier, err := svix.NewWebhook(c.BluedotSecret)
	if err != nil {
		return nil, fmt.Errorf("initing bluedot webhook verifier: %w", err)
	}

	endpoints := []endpoint{
		&transcripts{
			verifier: bluedotVerifier,
			service:  s.Transcripts,
		},
		&notifications{
			service: s.Notifications,
		},
	}

	mux := http.NewServeMux()
	for _, e := range endpoints {
		e.handleRoutes(mux.HandleFunc)
	}

	return &http.Server{
		Addr:              c.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}
