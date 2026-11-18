// Package audiodownload downloads individual public tracks as MP3 files.
package audiodownload

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// ErrInvalidURL indicates that the link does not identify a supported item.
var ErrInvalidURL = errors.New("audiodownload: invalid source URL")

var (
	youtubeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)
	slug      = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// NormalizeURL accepts YouTube videos, SoundCloud tracks and SoundCloud share
// links. It returns HTTPS URLs without tracking parameters or playlist context.
func NormalizeURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", ErrInvalidURL
	}

	// Use the escaped path so an encoded slash cannot change the link shape.
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(parsed.EscapedPath(), "/"), "/"), "/")
	host := strings.ToLower(parsed.Host)
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		switch {
		case len(parts) == 1 && parts[0] == "watch":
			query, err := url.ParseQuery(parsed.RawQuery)
			if err == nil && len(query["v"]) == 1 {
				return canonicalYouTubeURL(query.Get("v"))
			}
		case len(parts) == 2 && parts[0] == "shorts":
			return canonicalYouTubeURL(parts[1])
		}
	case "youtu.be":
		if len(parts) == 1 {
			return canonicalYouTubeURL(parts[0])
		}
	case "soundcloud.com", "www.soundcloud.com", "m.soundcloud.com":
		if len(parts) == 2 && slug.MatchString(parts[0]) && slug.MatchString(parts[1]) {
			switch strings.ToLower(parts[1]) {
			case "tracks", "albums", "sets", "reposts", "likes", "spotlight", "comments":
				return "", ErrInvalidURL
			}
			if parts[0] == "stations" && parts[1] == "track" {
				return "", ErrInvalidURL
			}
			return "https://soundcloud.com/" + parts[0] + "/" + parts[1], nil
		}
	case "on.soundcloud.com", "soundcloud.app.goo.gl":
		if len(parts) == 1 && slug.MatchString(parts[0]) {
			return "https://" + host + "/" + parts[0], nil
		}
	}
	return "", ErrInvalidURL
}

func canonicalYouTubeURL(id string) (string, error) {
	if !youtubeID.MatchString(id) {
		return "", ErrInvalidURL
	}
	return "https://www.youtube.com/watch?v=" + id, nil
}

func isSoundCloudShareURL(canonical string) bool {
	return strings.HasPrefix(canonical, "https://on.soundcloud.com/") ||
		strings.HasPrefix(canonical, "https://soundcloud.app.goo.gl/")
}

var soundCloudRedirectClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func resolveSoundCloudURL(ctx context.Context, client *http.Client, canonical string) (string, error) {
	for range 3 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, canonical, nil)
		if err != nil {
			return "", ErrInvalidURL
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", ErrDownloadFailed
		}
		_ = resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
			http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		default:
			return "", ErrDownloadFailed
		}

		location := resp.Header.Get("Location")
		if location == "" {
			return "", ErrInvalidURL
		}
		target, err := req.URL.Parse(location)
		if err != nil || target.Scheme != "https" {
			return "", ErrInvalidURL
		}
		canonical, err = NormalizeURL(target.String())
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(canonical, "https://soundcloud.com/") {
			return canonical, nil
		}
		if !isSoundCloudShareURL(canonical) {
			return "", ErrInvalidURL
		}
	}
	return "", ErrInvalidURL
}
