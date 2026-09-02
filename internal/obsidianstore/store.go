package obsidianstore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ollevche/marko/pkg/obsidian"
)

type Config struct {
	obsidian.Config
	TranscriptsPathPrefix   string
	NotificationsPathPrefix string
}

type Store struct {
	c                   *obsidian.Client
	transcriptFolders   []string
	notificationFolders []string
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
	s.notificationFolders = strings.Split(c.NotificationsPathPrefix, "/")
	return s, nil
}

func (s *Store) Close(ctx context.Context) error {
	return s.c.Close(ctx)
}

func (s *Store) readMarkdownFile(ctx context.Context, fp obsidian.FilePath) (obsidian.MarkdownFile, error) {
	b, err := s.c.ReadFile(ctx, fp)
	if errors.Is(err, obsidian.ErrFileNotFound) {
		return obsidian.MarkdownFile{}, nil
	}
	if err != nil {
		return obsidian.MarkdownFile{}, fmt.Errorf("reading %v: %w", fp, err)
	}

	return b, nil
}
