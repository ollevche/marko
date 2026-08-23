package obsidianstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/ollevche/marko/pkg/obsidian"
)

type Config struct {
	obsidian.Config
	TranscriptsPathPrefix string
}

type Store struct {
	c                 *obsidian.Client
	transcriptFolders []string
}

func New(ctx context.Context, c Config) (*Store, error) {
	client, err := obsidian.NewClient(ctx, c.Config)
	if err != nil {
		return nil, fmt.Errorf("creating obsidian client: %w", err)
	}

	s := &Store{
		c: client,
	}

	// this will fail once path prefix contains escaped slash
	s.transcriptFolders = strings.Split(c.TranscriptsPathPrefix, "/")
	return s, nil
}

func (s *Store) Close(ctx context.Context) error {
	return s.c.Close(ctx)
}
