package transcript

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

type Line struct {
	Speaker string
	Text    string
}

type Transcript struct {
	MeetingTime time.Time
	Duration    time.Duration
	Title       string
	Attendees   []string
	Lines       []Line
}

type Storage interface {
	UploadTranscript(context.Context, Transcript) error
}

type Service struct {
	UserLocation *time.Location
	Storage      Storage
}

var ErrEmptyTranscript = errors.New("no transcript lines")

func (s *Service) ProcessTranscript(ctx context.Context, t Transcript) error {
	if len(t.Lines) == 0 {
		return ErrEmptyTranscript
	}

	now := time.Now().In(s.UserLocation)

	if t.MeetingTime.IsZero() {
		t.MeetingTime = now.Add(-1 * t.Duration)
	}

	t.MeetingTime = t.MeetingTime.In(s.UserLocation)

	if t.Duration == 0 {
		t.Duration = now.Sub(t.MeetingTime)
	}

	// just in case, it is usually non-empty
	if t.Title == "" {
		t.Title = "no title"
	}

	if len(t.Attendees) == 0 {
		att := map[string]struct{}{}
		for _, l := range t.Lines {
			att[l.Speaker] = struct{}{}
		}
		for att := range att {
			t.Attendees = append(t.Attendees, att)
		}
		sort.Strings(t.Attendees)
	}

	if err := s.Storage.UploadTranscript(ctx, t); err != nil {
		return fmt.Errorf("uploading transcript: %w", err)
	}

	return nil
}
