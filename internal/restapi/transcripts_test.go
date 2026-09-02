package restapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ollevche/marko/internal/domain/transcript"
	svix "github.com/svix/svix-webhooks/go"
)

var testSecret = "whsec_" + base64.StdEncoding.EncodeToString([]byte("marko-webhook-test-secret"))

// fakeService records the domain transcripts the handler decodes.
type fakeService struct {
	got []transcript.Transcript
	err error
}

func (f *fakeService) ProcessTranscript(_ context.Context, t transcript.Transcript) error {
	f.got = append(f.got, t)
	return f.err
}

// signedRequest builds a request carrying headers the svix verifier accepts.
func signedRequest(t *testing.T, payload string) *http.Request {
	t.Helper()

	wh, err := svix.NewWebhook(testSecret)
	if err != nil {
		t.Fatalf("NewWebhook() error = %v", err)
	}

	const msgID = "msg_test"
	ts := time.Now()

	sig, err := wh.Sign(msgID, ts, []byte(payload))
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/meetings/transcripts", bytes.NewReader([]byte(payload)))
	req.Header.Set("svix-id", msgID)
	req.Header.Set("svix-timestamp", strconv.FormatInt(ts.Unix(), 10))
	req.Header.Set("svix-signature", sig)
	return req
}

func serve(t *testing.T, svc TranscriptService, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	srv, err := BuildServer(Config{Addr: ":0", BluedotSecret: testSecret}, Services{Transcripts: svc})
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}

	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	return rec
}

const fullPayload = `{
	"attendees": ["Alice", "Bob"],
	"createdAt": 1755903600,
	"duration": 1800.5,
	"meetingId": "mtg_1",
	"meetingTime": 1755900000,
	"owner": {"email": "ollevche@gmail.com"},
	"title": "Weekly sync",
	"transcript": [
		{"speaker": "Alice", "text": "hello"},
		{"speaker": "Bob", "text": "hi"},
		{"speaker": "Alice", "text": "bye"}
	],
	"type": "transcript.created",
	"videoId": "vid_1"
}`

func TestPostMeetingTranscriptDecodesBody(t *testing.T) {
	svc := &fakeService{}

	rec := serve(t, svc, signedRequest(t, fullPayload))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body)
	}
	if len(svc.got) != 1 {
		t.Fatalf("service called %d times, want 1", len(svc.got))
	}
	got := svc.got[0]

	// The lines must match the payload exactly: an earlier revision pre-sized the
	// slice and then appended, prefixing every note with blank speakers.
	want := []transcript.Line{
		{Speaker: "Alice", Text: "hello"},
		{Speaker: "Bob", Text: "hi"},
		{Speaker: "Alice", Text: "bye"},
	}
	if len(got.Lines) != len(want) {
		t.Fatalf("Lines has %d entries, want %d: %+v", len(got.Lines), len(want), got.Lines)
	}
	for i := range want {
		if got.Lines[i] != want[i] {
			t.Errorf("Lines[%d] = %+v, want %+v", i, got.Lines[i], want[i])
		}
	}

	if got.Title != "Weekly sync" {
		t.Errorf("Title = %q, want %q", got.Title, "Weekly sync")
	}
	if !got.MeetingTime.Equal(time.Unix(1755900000, 0)) {
		t.Errorf("MeetingTime = %v, want %v", got.MeetingTime, time.Unix(1755900000, 0))
	}
	// Fractional seconds are truncated, not rounded.
	if got.Duration != 1800*time.Second {
		t.Errorf("Duration = %v, want %v", got.Duration, 1800*time.Second)
	}
	if len(got.Attendees) != 2 || got.Attendees[0] != "Alice" || got.Attendees[1] != "Bob" {
		t.Errorf("Attendees = %q, want [Alice Bob]", got.Attendees)
	}
}

// Fields Bluedot may omit must decode to their zero values, which is what the
// domain service keys its defaulting off.
func TestPostMeetingTranscriptDecodesMinimalBody(t *testing.T) {
	svc := &fakeService{}

	rec := serve(t, svc, signedRequest(t, `{"transcript":[{"speaker":"Alice","text":"hello"}]}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body)
	}
	got := svc.got[0]

	if !got.MeetingTime.IsZero() {
		t.Errorf("MeetingTime = %v, want the zero time", got.MeetingTime)
	}
	if got.Duration != 0 {
		t.Errorf("Duration = %v, want 0", got.Duration)
	}
	if got.Title != "" {
		t.Errorf("Title = %q, want empty", got.Title)
	}
	if got.Attendees != nil {
		t.Errorf("Attendees = %q, want nil", got.Attendees)
	}
	if len(got.Lines) != 1 {
		t.Errorf("Lines has %d entries, want 1: %+v", len(got.Lines), got.Lines)
	}
}

func TestPostMeetingTranscriptEmptyTranscriptDecodesToNoLines(t *testing.T) {
	svc := &fakeService{}

	rec := serve(t, svc, signedRequest(t, `{"title":"empty","transcript":[]}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(svc.got[0].Lines) != 0 {
		t.Errorf("Lines = %+v, want none", svc.got[0].Lines)
	}
}

// An empty transcript is a permanent condition, so it is absorbed into a 200
// rather than inviting Bluedot to redeliver it.
func TestPostMeetingTranscriptAbsorbsEmptyTranscriptError(t *testing.T) {
	svc := &fakeService{err: transcript.ErrEmptyTranscript}

	rec := serve(t, svc, signedRequest(t, fullPayload))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestPostMeetingTranscriptReportsServiceFailure(t *testing.T) {
	svc := &fakeService{err: errors.New("vault is on fire")}

	rec := serve(t, svc, signedRequest(t, fullPayload))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestPostMeetingTranscriptRejectsMalformedJSON(t *testing.T) {
	svc := &fakeService{}

	rec := serve(t, svc, signedRequest(t, `{"transcript": [`))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if len(svc.got) != 0 {
		t.Errorf("service was called %d times for a malformed body, want 0", len(svc.got))
	}
}

func TestPostMeetingTranscriptRejectsUnverifiedRequests(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*http.Request)
		wantMsg string
	}{
		{
			name:    "no signature headers",
			mutate:  func(r *http.Request) { r.Header.Del("svix-signature") },
			wantMsg: "missing headers",
		},
		{
			name:    "wrong signature",
			mutate:  func(r *http.Request) { r.Header.Set("svix-signature", "v1,ZGVhZGJlZWY=") },
			wantMsg: "forged signature",
		},
		{
			name: "signature does not cover this body",
			mutate: func(r *http.Request) {
				r.Body = io.NopCloser(strings.NewReader(`{"title":"tampered"}`))
			},
			wantMsg: "tampered payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{}
			req := signedRequest(t, fullPayload)
			tt.mutate(req)

			rec := serve(t, svc, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status for %s = %d, want %d", tt.wantMsg, rec.Code, http.StatusUnauthorized)
			}
			if len(svc.got) != 0 {
				t.Errorf("service was called for %s, want no call", tt.wantMsg)
			}
		})
	}
}

func TestTranscriptRouteRejectsOtherMethods(t *testing.T) {
	svc := &fakeService{}
	req := httptest.NewRequest(http.MethodGet, "/meetings/transcripts", nil)

	rec := serve(t, svc, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
