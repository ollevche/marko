package obsidian

import (
	"fmt"
	"strings"

	"github.com/ollevche/marko/pkg/obsidian"
)

type Config struct {
	obsidian.Config
	TranscriptsPathPrefix string
}

type Storage struct {
	c                 *obsidian.Client
	transcriptFolders []string
}

func NewStorage(c Config) (*Storage, error) {
	client, err := obsidian.NewClient(c.Config)
	if err != nil {
		return nil, fmt.Errorf("creating obsidian client: %w", err)
	}

	s := &Storage{
		c: client,
	}

	// this will fail once path prefix contains escapeed slash
	s.transcriptFolders = strings.Split(c.TranscriptsPathPrefix, "/")
	return s, nil
}

func (s *Storage) Close() error {
	return s.c.Close()
}
