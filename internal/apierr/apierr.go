// Package apierr defines sentinel errors shared across the OpenAI-backed
// subsystems (stt, cleanup) so the application layer can identify
// category-specific failures (e.g. exhausted credits) and react with a
// user-visible notification instead of a generic log line.
package apierr

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrInsufficientCredits indicates the OpenAI API refused a call because
// the account is out of quota or credits. It is wrapped by HTTP callers
// when the response is 402 Payment Required, or 429 with an OpenAI error
// body of type/code "insufficient_quota".
var ErrInsufficientCredits = errors.New("OpenAI insufficient credits")

// IsInsufficientCredits reports whether an OpenAI HTTP response indicates an
// exhausted-quota / payment-required condition (as opposed to a transient
// rate limit). 402 always qualifies; 429 only qualifies when the JSON body's
// error.code or error.type is "insufficient_quota" — a plain 429 is a
// throttling event and should remain a generic error.
func IsInsufficientCredits(statusCode int, body []byte) bool {
	if statusCode == http.StatusPaymentRequired {
		return true
	}
	if statusCode != http.StatusTooManyRequests {
		return false
	}
	var parsed struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	return parsed.Error.Code == "insufficient_quota" || parsed.Error.Type == "insufficient_quota"
}
