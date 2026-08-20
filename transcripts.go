package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

type transcriptHandler struct {
	verifier *svix.Webhook
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

	log.Println(t.title())
	log.Println(t.body())

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

func (t transcript) title() string {
	return fmt.Sprintf("%v-%v %s", t.MeetingTime.Hour(), t.MeetingTime.Minute(), t.Title)
}

func (t transcript) body() string {
	var b strings.Builder

	b.WriteString("Owner: ")
	b.WriteString(t.Owner.Email)
	b.WriteByte('\n')

	b.WriteString("Date: ")
	b.WriteString(t.MeetingTime.Format(time.DateTime))
	b.WriteByte('\n')

	b.WriteString("Length: ")
	b.WriteString(time.Duration(int64(t.Duration) * int64(time.Second)).String())
	b.WriteByte('\n')

	b.WriteString("Attendees: ")
	b.WriteString(strings.Join(t.Attendees, ", "))
	b.WriteByte('\n')

	b.WriteString("\n---\n")

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
