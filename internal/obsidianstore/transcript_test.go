package obsidianstore

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ollevche/marko/internal/domain/transcript"
)

// kyiv stands in for the user location the domain service applies before the
// transcript reaches storage, as a fixed zone so the tests do not need tzdata.
var kyiv = time.FixedZone("EET", 2*60*60)

// prefix mirrors what New() derives from OBSIDIAN_TRANSCRIPTS_PATH_PREFIX.
var prefix = []string{"marko", "transcripts"}

func meetingAt(y int, m time.Month, d, hour, min int) time.Time {
	return time.Date(y, m, d, hour, min, 0, 0, kyiv)
}

func aTranscript() transcript.Transcript {
	return transcript.Transcript{
		MeetingTime: meetingAt(2026, time.August, 23, 14, 5),
		Duration:    90 * time.Minute,
		Title:       "Weekly sync",
		Attendees:   []string{"Alice", "Bob"},
		Lines: []transcript.Line{
			{Speaker: "Alice", Text: "hello"},
			{Speaker: "Bob", Text: "hi there"},
		},
	}
}

func TestNewMarkdownFileFromTranscriptFolders(t *testing.T) {
	tests := []struct {
		name    string
		prefix  []string
		meeting time.Time
		want    []string
	}{
		{
			name:    "prefix then year, month and day",
			prefix:  prefix,
			meeting: meetingAt(2026, time.August, 23, 14, 5),
			want:    []string{"marko", "transcripts", "2026", "08 - August", "23 - Sunday"},
		},
		{
			// Single digits are padded so the folders sort in Obsidian's file tree.
			name:    "single digit month and day are padded",
			prefix:  prefix,
			meeting: meetingAt(2026, time.January, 5, 9, 0),
			want:    []string{"marko", "transcripts", "2026", "01 - January", "05 - Monday"},
		},
		{
			name:    "double digit month and day",
			prefix:  prefix,
			meeting: meetingAt(2026, time.October, 15, 9, 0),
			want:    []string{"marko", "transcripts", "2026", "10 - October", "15 - Thursday"},
		},
		{
			name:    "no prefix leaves only the date folders",
			prefix:  nil,
			meeting: meetingAt(2026, time.December, 31, 23, 59),
			want:    []string{"2026", "12 - December", "31 - Thursday"},
		},
		{
			name:    "single segment prefix",
			prefix:  []string{"inbox"},
			meeting: meetingAt(2026, time.August, 23, 14, 5),
			want:    []string{"inbox", "2026", "08 - August", "23 - Sunday"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := aTranscript()
			in.MeetingTime = tt.meeting

			got := newMarkdownFileFromTranscript(tt.prefix, in)

			if !slices.Equal(got.Folders, tt.want) {
				t.Errorf("Folders = %q, want %q", got.Folders, tt.want)
			}
		})
	}
}

func TestNewMarkdownFileFromTranscriptFilename(t *testing.T) {
	tests := []struct {
		name    string
		meeting time.Time
		title   string
		want    string
	}{
		{
			name:    "hour and minute then title",
			meeting: meetingAt(2026, time.August, 23, 14, 5),
			title:   "Weekly sync",
			want:    "14-05 Weekly sync.md",
		},
		{
			name:    "midnight",
			meeting: meetingAt(2026, time.August, 23, 0, 0),
			title:   "Late night",
			want:    "00-00 Late night.md",
		},
		{
			name:    "last minute of the day",
			meeting: meetingAt(2026, time.August, 23, 23, 59),
			title:   "Retro",
			want:    "23-59 Retro.md",
		},
		{
			// Path-hostile titles survive here; pkg/obsidian sanitizes on write.
			name:    "title is not sanitized at this layer",
			meeting: meetingAt(2026, time.August, 23, 9, 30),
			title:   "Q3/Q4: planning",
			want:    "09-30 Q3/Q4: planning.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := aTranscript()
			in.MeetingTime = tt.meeting
			in.Title = tt.title

			got := newMarkdownFileFromTranscript(prefix, in)

			if got.Filename != tt.want {
				t.Errorf("Filename = %q, want %q", got.Filename, tt.want)
			}
		})
	}
}

func TestNewMarkdownFileFromTranscriptContent(t *testing.T) {
	got := newMarkdownFileFromTranscript(prefix, aTranscript())

	want := "Date: 2026-08-23 14:05:00\n" +
		"Duration: 1h30m0s\n" +
		"Attendees: Alice, Bob\n" +
		"\n" +
		"Alice\n" +
		"hello\n" +
		"\n" +
		"Bob\n" +
		"hi there"

	if got.Content != want {
		t.Errorf("Content =\n%q\nwant\n%q", got.Content, want)
	}
}

func TestNewMarkdownFileFromTranscriptContentHeader(t *testing.T) {
	tests := []struct {
		name      string
		duration  time.Duration
		attendees []string
		want      string
	}{
		{
			name:      "attendees joined with commas",
			duration:  90 * time.Minute,
			attendees: []string{"Alice", "Bob", "Carol"},
			want:      "Date: 2026-08-23 14:05:00\nDuration: 1h30m0s\nAttendees: Alice, Bob, Carol\n\n",
		},
		{
			name:      "no attendees falls back to n/a",
			duration:  90 * time.Minute,
			attendees: nil,
			want:      "Date: 2026-08-23 14:05:00\nDuration: 1h30m0s\nAttendees: n/a\n\n",
		},
		{
			name:      "empty attendee slice also falls back",
			duration:  45 * time.Second,
			attendees: []string{},
			want:      "Date: 2026-08-23 14:05:00\nDuration: 45s\nAttendees: n/a\n\n",
		},
		{
			name:      "zero duration",
			duration:  0,
			attendees: []string{"Alice"},
			want:      "Date: 2026-08-23 14:05:00\nDuration: 0s\nAttendees: Alice\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := aTranscript()
			in.Duration = tt.duration
			in.Attendees = tt.attendees
			in.Lines = nil

			got := newMarkdownFileFromTranscript(prefix, in)

			// With no lines the content is exactly the header block.
			if got.Content != tt.want {
				t.Errorf("Content = %q, want %q", got.Content, tt.want)
			}
		})
	}
}

// Turns are separated by a blank line, and the note does not end with one.
func TestNewMarkdownFileFromTranscriptLineSeparators(t *testing.T) {
	const header = "Date: 2026-08-23 14:05:00\nDuration: 1h30m0s\nAttendees: Alice, Bob\n\n"

	tests := []struct {
		name  string
		lines []transcript.Line
		want  string
	}{
		{
			name:  "single line has no trailing blank line",
			lines: []transcript.Line{{Speaker: "Alice", Text: "hello"}},
			want:  header + "Alice\nhello",
		},
		{
			name: "three lines are separated by blank lines",
			lines: []transcript.Line{
				{Speaker: "Alice", Text: "one"},
				{Speaker: "Bob", Text: "two"},
				{Speaker: "Alice", Text: "three"},
			},
			want: header + "Alice\none\n\nBob\ntwo\n\nAlice\nthree",
		},
		{
			name: "multi-line text is kept verbatim",
			lines: []transcript.Line{
				{Speaker: "Alice", Text: "first\nsecond"},
				{Speaker: "Bob", Text: "ok"},
			},
			want: header + "Alice\nfirst\nsecond\n\nBob\nok",
		},
		{
			name:  "no lines leaves just the header",
			lines: nil,
			want:  header,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := aTranscript()
			in.Lines = tt.lines

			got := newMarkdownFileFromTranscript(prefix, in)

			if got.Content != tt.want {
				t.Errorf("Content = %q, want %q", got.Content, tt.want)
			}
		})
	}
}

// Everything is derived from the wall clock of MeetingTime's own location, so
// the same instant files differently depending on the zone the domain applied.
func TestNewMarkdownFileFromTranscriptUsesMeetingTimeLocation(t *testing.T) {
	instant := time.Date(2026, time.August, 23, 0, 30, 0, 0, time.UTC)

	in := aTranscript()
	in.MeetingTime = instant
	utc := newMarkdownFileFromTranscript(prefix, in)

	in.MeetingTime = instant.In(kyiv)
	local := newMarkdownFileFromTranscript(prefix, in)

	if utc.Filename != "00-30 Weekly sync.md" {
		t.Errorf("UTC filename = %q, want %q", utc.Filename, "00-30 Weekly sync.md")
	}
	if local.Filename != "02-30 Weekly sync.md" {
		t.Errorf("local filename = %q, want %q", local.Filename, "02-30 Weekly sync.md")
	}
	if got := local.Folders[len(local.Folders)-1]; got != "23 - Sunday" {
		t.Errorf("local day folder = %q, want %q", got, "23 - Sunday")
	}
	if !strings.HasPrefix(local.Content, "Date: 2026-08-23 02:30:00\n") {
		t.Errorf("local content starts with %q, want the local wall clock", firstLine(local.Content))
	}
}

func firstLine(s string) string {
	for i := range s {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
