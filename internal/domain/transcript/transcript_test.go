package transcript_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/ollevche/marko/internal/domain/transcript"
)

// kyiv stands in for the configured user location, as a fixed zone so the tests
// do not depend on the tzdata of whatever machine runs them.
var kyiv = time.FixedZone("EET", 2*60*60)

// clockTolerance absorbs the wall-clock drift between the time.Now() inside
// ProcessTranscript and the one the test compares against.
const clockTolerance = 5 * time.Second

// fakeStore records what ProcessTranscript hands down to storage.
type fakeStore struct {
	got []transcript.Transcript
	err error
}

func (f *fakeStore) UploadTranscript(_ context.Context, t transcript.Transcript) error {
	f.got = append(f.got, t)
	return f.err
}

// process runs the service and returns the single transcript that reached storage.
func process(t *testing.T, in transcript.Transcript) transcript.Transcript {
	t.Helper()

	store := &fakeStore{}
	svc := &transcript.Service{UserLocation: kyiv, Store: store}

	if err := svc.ProcessTranscript(context.Background(), in); err != nil {
		t.Fatalf("ProcessTranscript() error = %v, want nil", err)
	}
	if len(store.got) != 1 {
		t.Fatalf("storage received %d transcripts, want 1", len(store.got))
	}
	return store.got[0]
}

func someLines(speakers ...string) []transcript.Line {
	out := make([]transcript.Line, 0, len(speakers))
	for _, s := range speakers {
		out = append(out, transcript.Line{Speaker: s, Text: "..."})
	}
	return out
}

func assertNearTime(t *testing.T, label string, got, want time.Time) {
	t.Helper()
	if d := got.Sub(want); d < -clockTolerance || d > clockTolerance {
		t.Errorf("%s = %v, want within %v of %v (off by %v)", label, got, clockTolerance, want, d)
	}
}

func assertNearDuration(t *testing.T, label string, got, want time.Duration) {
	t.Helper()
	if d := got - want; d < -clockTolerance || d > clockTolerance {
		t.Errorf("%s = %v, want within %v of %v", label, got, clockTolerance, want)
	}
}

func TestProcessTranscriptRejectsEmptyTranscript(t *testing.T) {
	store := &fakeStore{}
	svc := &transcript.Service{UserLocation: kyiv, Store: store}

	err := svc.ProcessTranscript(context.Background(), transcript.Transcript{Title: "empty"})

	if !errors.Is(err, transcript.ErrEmptyTranscript) {
		t.Errorf("error = %v, want ErrEmptyTranscript", err)
	}
	if len(store.got) != 0 {
		t.Errorf("storage was called %d times for an empty transcript, want 0", len(store.got))
	}
}

// The note's folder tree and filename are derived from the wall-clock fields of
// MeetingTime, so it has to arrive at storage in the user's zone, not UTC.
func TestProcessTranscriptConvertsMeetingTimeToUserLocation(t *testing.T) {
	// 00:30 UTC is 02:30 in a +02:00 zone - and the day-of-month differs too.
	utcTime := time.Date(2026, time.August, 23, 0, 30, 0, 0, time.UTC)

	got := process(t, transcript.Transcript{
		MeetingTime: utcTime,
		Duration:    time.Hour,
		Lines:       someLines("Alice"),
	})

	if !got.MeetingTime.Equal(utcTime) {
		t.Errorf("MeetingTime = %v, want the same instant as %v", got.MeetingTime, utcTime)
	}
	if got.MeetingTime.Location().String() != kyiv.String() {
		t.Errorf("MeetingTime location = %v, want %v", got.MeetingTime.Location(), kyiv)
	}
	if h, m := got.MeetingTime.Hour(), got.MeetingTime.Minute(); h != 2 || m != 30 {
		t.Errorf("local clock = %02d:%02d, want 02:30", h, m)
	}
	if d := got.MeetingTime.Day(); d != 23 {
		t.Errorf("local day = %d, want 23", d)
	}
}

func TestProcessTranscriptFillsMissingTimes(t *testing.T) {
	tests := []struct {
		name         string
		meetingTime  time.Time
		duration     time.Duration
		wantMeeting  func(now time.Time) time.Time
		wantDuration time.Duration
	}{
		{
			name:         "meeting time derived from duration",
			duration:     45 * time.Minute,
			wantMeeting:  func(now time.Time) time.Time { return now.Add(-45 * time.Minute) },
			wantDuration: 45 * time.Minute,
		},
		{
			name:         "duration derived from meeting time",
			meetingTime:  time.Now().Add(-30 * time.Minute),
			wantMeeting:  func(now time.Time) time.Time { return now.Add(-30 * time.Minute) },
			wantDuration: 30 * time.Minute,
		},
		{
			name:         "both missing",
			wantMeeting:  func(now time.Time) time.Time { return now },
			wantDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()

			got := process(t, transcript.Transcript{
				MeetingTime: tt.meetingTime,
				Duration:    tt.duration,
				Lines:       someLines("Alice"),
			})

			assertNearTime(t, "MeetingTime", got.MeetingTime, tt.wantMeeting(now))
			assertNearDuration(t, "Duration", got.Duration, tt.wantDuration)
			if got.MeetingTime.Location().String() != kyiv.String() {
				t.Errorf("MeetingTime location = %v, want %v", got.MeetingTime.Location(), kyiv)
			}
		})
	}
}

func TestProcessTranscriptDefaultsTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{in: "", want: "no title"},
		{in: "Weekly sync", want: "Weekly sync"},
	}

	for _, tt := range tests {
		got := process(t, transcript.Transcript{
			MeetingTime: time.Now(),
			Duration:    time.Hour,
			Title:       tt.in,
			Lines:       someLines("Alice"),
		})

		if got.Title != tt.want {
			t.Errorf("Title for %q = %q, want %q", tt.in, got.Title, tt.want)
		}
	}
}

func TestProcessTranscriptDerivesAttendeesFromSpeakers(t *testing.T) {
	got := process(t, transcript.Transcript{
		MeetingTime: time.Now(),
		Duration:    time.Hour,
		Lines:       someLines("Bob", "Alice", "Bob", "Carol", "Alice"),
	})

	want := []string{"Alice", "Bob", "Carol"}
	if !slices.Equal(got.Attendees, want) {
		t.Errorf("Attendees = %q, want %q (deduplicated and sorted)", got.Attendees, want)
	}
}

func TestProcessTranscriptKeepsProvidedAttendees(t *testing.T) {
	// Deliberately unsorted, and disjoint from the speakers: what the webhook
	// reports wins over what the lines imply.
	provided := []string{"Zoe", "Adam"}

	got := process(t, transcript.Transcript{
		MeetingTime: time.Now(),
		Duration:    time.Hour,
		Attendees:   provided,
		Lines:       someLines("Bob", "Alice"),
	})

	if !slices.Equal(got.Attendees, provided) {
		t.Errorf("Attendees = %q, want %q untouched", got.Attendees, provided)
	}
}

func TestProcessTranscriptWrapsStoreError(t *testing.T) {
	storeErr := errors.New("vault is on fire")
	store := &fakeStore{err: storeErr}
	svc := &transcript.Service{UserLocation: kyiv, Store: store}

	err := svc.ProcessTranscript(context.Background(), transcript.Transcript{
		MeetingTime: time.Now(),
		Duration:    time.Hour,
		Lines:       someLines("Alice"),
	})

	if !errors.Is(err, storeErr) {
		t.Errorf("error = %v, want it to wrap %v", err, storeErr)
	}
	if errors.Is(err, transcript.ErrEmptyTranscript) {
		t.Error("a store failure must not be reported as ErrEmptyTranscript")
	}
}
