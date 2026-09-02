package restapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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

func withOverriddenCancel(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithoutCancel(r.Context())

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		next(w, r.WithContext(ctx))
	}
}
