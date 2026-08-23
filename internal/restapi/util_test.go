package restapi

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnixSecondsUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{name: "bare number", in: `1755900000`, want: time.Unix(1755900000, 0)},
		{name: "quoted number", in: `"1755900000"`, want: time.Unix(1755900000, 0)},
		{name: "zero", in: `0`, want: time.Unix(0, 0)},
		{name: "negative", in: `-5`, want: time.Unix(-5, 0)},
		{name: "null", in: `null`, want: time.Time{}},
		{name: "empty string", in: `""`, want: time.Time{}},
		// Bluedot sends whole seconds; anything else is rejected rather than rounded.
		{name: "fractional", in: `1755900000.5`, wantErr: true},
		{name: "not a number", in: `"tomorrow"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got unixSeconds
			err := json.Unmarshal([]byte(tt.in), &got)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) error = nil, want an error (got %v)", tt.in, got.Time)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) error = %v, want nil", tt.in, err)
			}
			if !got.Time.Equal(tt.want) {
				t.Errorf("Unmarshal(%s) = %v, want %v", tt.in, got.Time, tt.want)
			}
		})
	}
}

// A missing key must leave the zero value behind, since ProcessTranscript keys
// its meeting-time defaulting off IsZero.
func TestUnixSecondsAbsentFieldStaysZero(t *testing.T) {
	var body struct {
		MeetingTime unixSeconds `json:"meetingTime"`
	}

	if err := json.Unmarshal([]byte(`{}`), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !body.MeetingTime.IsZero() {
		t.Errorf("MeetingTime = %v, want the zero time", body.MeetingTime.Time)
	}
}

func TestUnixSecondsMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   unixSeconds
		want string
	}{
		{name: "zero is null", in: unixSeconds{}, want: `null`},
		{name: "value is a bare number", in: unixSeconds{time.Unix(1755900000, 0)}, want: `1755900000`},
		// Sub-second precision is dropped, matching the wire format.
		{name: "nanoseconds truncated", in: unixSeconds{time.Unix(1755900000, 999)}, want: `1755900000`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestUnixSecondsRoundTrip(t *testing.T) {
	want := unixSeconds{time.Unix(1755900000, 0)}

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got unixSeconds
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", encoded, err)
	}
	if !got.Time.Equal(want.Time) {
		t.Errorf("round trip = %v, want %v", got.Time, want.Time)
	}
}
