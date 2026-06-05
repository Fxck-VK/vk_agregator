// Package miniapp implements the VK Mini App BFF inbound adapter. It verifies
// VK launch-params signatures and exposes a thin HTTP API over the existing
// service layer. It never calls AI providers directly.
package miniapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrMissingSign is returned when the sign parameter is absent.
var ErrMissingSign = errors.New("miniapp: missing sign parameter")

// ErrInvalidSign is returned when the signature does not match.
var ErrInvalidSign = errors.New("miniapp: invalid signature")

// ErrExpiredParams is returned when vk_ts is older than the allowed max age.
var ErrExpiredParams = errors.New("miniapp: launch params expired")

// ErrMissingTimestamp is returned when vk_ts is required but absent.
var ErrMissingTimestamp = errors.New("miniapp: missing vk_ts")

// ErrInvalidTimestamp is returned when vk_ts cannot be trusted.
var ErrInvalidTimestamp = errors.New("miniapp: invalid vk_ts")

// ErrMissingUserID is returned when vk_user_id is absent or zero.
var ErrMissingUserID = errors.New("miniapp: missing vk_user_id")

// VerifyLaunchParams validates a VK Mini App launch-params query string.
//
// The algorithm (from https://dev.vk.com/mini-apps/development/launch-params-sign):
//  1. Filter params whose key starts with "vk_".
//  2. Sort filtered pairs by key (url.Values.Encode does this).
//  3. URL-encode as "key=value&..." (percent-encoding per RFC 3986).
//  4. Compute HMAC-SHA256 with appSecret.
//  5. Base64url-encode the digest (no padding).
//  6. Compare with the "sign" parameter using a constant-time comparison.
//
// When appSecret is empty the signature check is skipped and the function only
// parses and validates the required fields (dev/mock mode).
// When maxAge > 0 the vk_ts timestamp must be within maxAge of now.
//
// Returns the parsed params on success so callers can read vk_user_id etc.
func VerifyLaunchParams(rawQuery, appSecret string, maxAge time.Duration) (url.Values, error) {
	params, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("miniapp: invalid launch params: %w", err)
	}

	if appSecret != "" {
		gotSign := params.Get("sign")
		if gotSign == "" {
			return nil, ErrMissingSign
		}

		// Build the sorted vk_* params string.
		vkParams := make(url.Values)
		for k, v := range params {
			if strings.HasPrefix(k, "vk_") {
				vkParams[k] = v
			}
		}
		toSign := vkParams.Encode() // sorted + percent-encoded

		mac := hmac.New(sha256.New, []byte(appSecret))
		mac.Write([]byte(toSign))
		expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(expected), []byte(gotSign)) {
			return nil, ErrInvalidSign
		}
	}

	if maxAge > 0 {
		tsStr := params.Get("vk_ts")
		if tsStr == "" {
			return nil, ErrMissingTimestamp
		}
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			return nil, ErrInvalidTimestamp
		}
		age := time.Since(time.Unix(ts, 0))
		if age < 0 {
			return nil, ErrInvalidTimestamp
		}
		if age > maxAge {
			return nil, ErrExpiredParams
		}
	}

	if params.Get("vk_user_id") == "" {
		return nil, ErrMissingUserID
	}

	return params, nil
}

// VKUserIDFromParams extracts the vk_user_id as int64 from verified params.
func VKUserIDFromParams(params url.Values) (int64, error) {
	raw := params.Get("vk_user_id")
	if raw == "" {
		return 0, ErrMissingUserID
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("miniapp: invalid vk_user_id %q", raw)
	}
	return id, nil
}
