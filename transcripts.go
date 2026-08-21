package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

type transcriptHandler struct {
	verifier *svix.Webhook
	obsidian *obsidian
}

func (h *transcriptHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verification needs the exact raw bytes, so read the body once and reuse it.
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.verifier.Verify(payload, r.Header); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var t transcript

	if err := json.Unmarshal(payload, &t); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.obsidian.syncTranscript(t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

type transcript struct {
	Attendees   []string    `json:"attendees"`
	CreatedAt   unixSeconds `json:"createdAt"`
	Duration    float64     `json:"duration"`
	MeetingID   string      `json:"meetingId"`
	MeetingTime unixSeconds `json:"meetingTime"`
	Owner       struct {
		Email string `json:"email"`
	} `json:"owner"`
	Title      string `json:"title"`
	Transcript []struct {
		Speaker string `json:"speaker"`
		Text    string `json:"text"`
	} `json:"transcript"`
	Type    string `json:"type"`
	VideoID string `json:"videoId"`
}

// unixSeconds is a time.Time carried in JSON as a Unix timestamp in whole
// seconds, which is how Bluedot sends its dates. It reads a bare number or a
// quoted one, and always writes a bare number back.
type unixSeconds struct {
	time.Time
}

func (u *unixSeconds) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		u.Time = time.Time{}
		return nil
	}

	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid unix timestamp %s: %w", data, err)
	}

	u.Time = time.Unix(secs, 0)
	return nil
}

func (u unixSeconds) MarshalJSON() ([]byte, error) {
	if u.IsZero() {
		return []byte("null"), nil
	}
	return strconv.AppendInt(nil, u.Unix(), 10), nil
}

func (t transcript) filename() string {
	return sanitizeFilename(t.title()) + ".md"
}

// sanitizeFilename drops the characters that filesystems or Obsidian refuse to
// carry in a note name, and collapses whatever whitespace is left behind.
func sanitizeFilename(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f: // control characters
			return ' '
		case strings.ContainsRune(`\/:*?"<>|#^[]`, r):
			return ' '
		default:
			return r
		}
	}, name)

	cleaned = strings.Join(strings.Fields(cleaned), " ")
	// Leading dots hide the file, trailing dots are dropped by some filesystems.
	cleaned = strings.Trim(cleaned, ".")

	if cleaned == "" {
		return "untitled"
	}

	return cleaned
}

var userLocation *time.Location

func init() {
	l, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		panic("loading user location: " + err.Error())
	}

	userLocation = l
}

func toUserLocation(t time.Time) time.Time {
	return t.In(userLocation)
}

func (t transcript) title() string {
	mt := toUserLocation(t.MeetingTime.Time)
	return fmt.Sprintf("%02d-%02d %s", mt.Hour(), mt.Minute(), t.Title)
}

func (t transcript) body() string {
	var b strings.Builder

	b.WriteString("Owner:\t\t")
	b.WriteString(t.Owner.Email)
	b.WriteByte('\n')

	mt := toUserLocation(t.MeetingTime.Time)

	b.WriteString("Date:\t\t\t")
	b.WriteString(mt.Format(time.DateTime))
	b.WriteByte('\n')

	b.WriteString("Length:\t\t")
	b.WriteString(time.Duration(int64(t.Duration) * int64(time.Second)).String())
	b.WriteByte('\n')

	b.WriteString("Attendees:\t")
	att := "<none>"
	if len(t.Attendees) != 0 {
		att = strings.Join(t.Attendees, ", ")
	}
	b.WriteString(att)
	b.WriteString("\n\n")

	for i, r := range t.Transcript {
		b.WriteString(r.Speaker)
		b.WriteByte('\n')

		b.WriteString(r.Text)
		if len(t.Transcript) != i+1 {
			b.WriteString("\n\n")
		}
	}

	return b.String()
}
