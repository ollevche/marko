package notification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Repository struct {
	URL      string
	FullName string
}

type Notification struct {
	ID     string
	Repo   Repository
	Title  string
	Type   string
	Unread bool
}

type State struct {
	Digest    string
	FetchedAt time.Time
}

type Summary struct {
	State
	Notifications []Notification
}

type Source interface {
	ListNotifications(context.Context) ([]Notification, error)
}

type Store interface {
	ReadNotificationState(context.Context) (State, error)
	UploadNotificationSummary(context.Context, Summary) error
}

type Service struct {
	Store Store

	m      sync.Mutex
	Source Source

	MinListingDelay time.Duration
}

func (s *Service) getMinListingDelay() time.Duration {
	if s.MinListingDelay == 0 {
		return time.Minute
	}
	return s.MinListingDelay
}

func (s *Service) ListAndUploadNotifications(ctx context.Context) error {
	if !s.m.TryLock() {
		return nil
	}
	defer s.m.Unlock()

	state, err := s.Store.ReadNotificationState(ctx)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}

	if state.FetchedAt.Add(s.getMinListingDelay()).After(time.Now().UTC()) {
		return nil
	}

	ns, err := s.Source.ListNotifications(ctx)
	if err != nil {
		return fmt.Errorf("listing notifications: %w", err)
	}

	digest := generateDigest(ns)
	if digest == state.Digest {
		return nil
	}

	summary := Summary{
		State: State{
			Digest:    digest,
			FetchedAt: time.Now().UTC(),
		},
		Notifications: ns,
	}

	if err := s.Store.UploadNotificationSummary(ctx, summary); err != nil {
		return fmt.Errorf("uploading summary: %w", err)
	}

	return nil
}

func generateDigest(n []Notification) string {
	metas := make([]string, len(n))

	for i, n := range n {
		metas[i] = strings.Join([]string{
			n.ID,
			n.Title,
			n.Type,
			n.Repo.FullName,
			n.Repo.URL,
			fmt.Sprintf("%v", n.Unread),
		}, ":")
	}

	metas = sort.StringSlice(metas)

	hash := sha256.Sum256([]byte(strings.Join(metas, ":")))

	return hex.EncodeToString(hash[:])
}
