package restapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
