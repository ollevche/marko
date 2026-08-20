package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	Attendees   []string  `json:"attendees"`
	CreatedAt   time.Time `json:"createdAt"`
	Duration    float64   `json:"duration"`
	MeetingID   time.Time `json:"meetingId"`
	MeetingTime time.Time `json:"meetingTime"`
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
