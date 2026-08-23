package restapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ollevche/marko/internal/domain/transcript"
	svix "github.com/svix/svix-webhooks/go"
)

type TranscriptService interface {
	ProcessTranscript(ctx context.Context, t transcript.Transcript) error
}

// uploadTimeout bounds a detached transcript upload, including the vault sync it
// triggers. It is deliberately longer than the server's WriteTimeout: finishing a
// half-written sync matters more than answering this particular delivery attempt.
const uploadTimeout = 2 * time.Minute

type transcripts struct {
	verifier *svix.Webhook
	service  TranscriptService
}

func (h *transcripts) handleRoutes(handle func(string, func(http.ResponseWriter, *http.Request))) {
	handle("POST /meetings/transcripts", h.postMeetingTranscript)
}

type postTranscriptBody struct {
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

func (h *transcripts) postMeetingTranscript(w http.ResponseWriter, r *http.Request) {
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

	var reqBody postTranscriptBody

	if err := json.Unmarshal(payload, &reqBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t := transcript.Transcript{
		MeetingTime: reqBody.MeetingTime.Time,
		Duration:    time.Duration(int(reqBody.Duration)) * time.Second,
		Title:       reqBody.Title,
		Attendees:   reqBody.Attendees,
		Lines:       make([]transcript.Line, len(reqBody.Transcript)),
	}
	for i, l := range reqBody.Transcript {
		t.Lines[i] = transcript.Line{
			Speaker: l.Speaker,
			Text:    l.Text,
		}
	}

	// Cancelling mid-sync would leave the vault half-written, so the upload runs
	// detached from the request and is bounded on its own.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), uploadTimeout)
	defer cancel()

	err = h.service.ProcessTranscript(ctx, t)
	if err != nil && !errors.Is(err, transcript.ErrEmptyTranscript) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}
