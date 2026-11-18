package audiodownload

import (
	"errors"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"youtube watch", " https://M.YOUTUBE.COM/watch?si=tracking&v=abcdefghijk&list=ignored#t=20 ", "https://www.youtube.com/watch?v=abcdefghijk"},
		{"youtube short hostname", "http://youtu.be/abcdefghijk?si=tracking", "https://www.youtube.com/watch?v=abcdefghijk"},
		{"youtube shorts", "https://www.youtube.com/shorts/abcdefghijk?si=tracking", "https://www.youtube.com/watch?v=abcdefghijk"},
		{"soundcloud track", "https://m.soundcloud.com/example-artist/example-track/?utm_source=copy#t=10", "https://soundcloud.com/example-artist/example-track"},
		{"soundcloud share link", "https://on.soundcloud.com/Example9?utm_source=copy", "https://on.soundcloud.com/Example9"},
		{"older soundcloud share link", "https://soundcloud.app.goo.gl/Example9", "https://soundcloud.app.goo.gl/Example9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeURL(tc.raw)
			if err != nil || got != tc.want {
				t.Fatalf("NormalizeURL() = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestNormalizeURLRejectsUnsupportedSources(t *testing.T) {
	for _, raw := range []string{
		"https://www.youtube.com.evil.example/watch?v=abcdefghijk",
		"https://www.youtube.com@evil.example/watch?v=abcdefghijk",
		"https://evil.example@www.youtube.com/watch?v=abcdefghijk",
		"https://www.youtube.com:8443/watch?v=abcdefghijk",
		"https://www.youtube.com/playlist?list=example",
		"https://www.youtube.com/watch?v=abcdefghijk&v=lmnopqrstuv",
		"https://youtu.be/abcdefghijk/other",
		"https://youtu.be/not-an-id",
		"https://soundcloud.com/example-artist/sets/example-set",
		"https://soundcloud.com/example-artist/likes",
		"https://soundcloud.com/example-artist/example-track/s-secret",
		"https://soundcloud.com/example-artist/example%2Ftrack",
		"https://on.soundcloud.com/Example9/Other",
		"ftp://soundcloud.com/example-artist/example-track",
		"https://127.0.0.1/example-artist/example-track",
	} {
		t.Run(raw, func(t *testing.T) {
			got, err := NormalizeURL(raw)
			if !errors.Is(err, ErrInvalidURL) || got != "" {
				t.Fatalf("NormalizeURL() = %q, %v, want ErrInvalidURL", got, err)
			}
		})
	}
}
