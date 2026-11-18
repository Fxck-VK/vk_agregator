package audiodownload

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	testYouTubeURL  = "https://www.youtube.com/watch?v=abcdefghijk"
	testProcessWait = 10 * time.Second
)

func TestDownloadUsesDedicatedExtractorAndCleansWorkDir(t *testing.T) {
	t.Setenv("AUDIODOWNLOAD_TEST_CREDENTIAL", "must stay in the worker")
	path := testExecutable(t, `
case " $* " in
  *" --use-extractors youtube "*) ;;
  *) exit 1 ;;
esac
case " $* " in
  *" -- https://www.youtube.com/watch?v=abcdefghijk "*) ;;
  *) exit 1 ;;
esac
test -z "${AUDIODOWNLOAD_TEST_CREDENTIAL+x}"
test "$HOME" -ef .
printf '%s' "$HOME" > "$0.cwd"
printf 'ID3example' > audio.mp3
`)
	data, err := New(path).Download(context.Background(), "https://youtu.be/abcdefghijk?si=removed")
	if err != nil || string(data) != "ID3example" {
		t.Fatalf("Download() = %q, %v", data, err)
	}
	workDir, err := os.ReadFile(path + ".cwd")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(workDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary working directory still exists: %v", err)
	}
}

func TestDownloadUsesSoundCloudTrackExtractor(t *testing.T) {
	path := testExecutable(t, `
case " $* " in
  *" --use-extractors soundcloud "*) ;;
  *) exit 1 ;;
esac
case " $* " in
  *" -- https://soundcloud.com/artist/track "*) ;;
  *) exit 1 ;;
esac
printf 'ID3soundcloud' > audio.mp3
`)
	data, err := New(path).Download(context.Background(), "https://m.soundcloud.com/artist/track?si=removed")
	if err != nil || string(data) != "ID3soundcloud" {
		t.Fatalf("Download() = %q, %v", data, err)
	}
}

func TestDownloadRejectsMissingOrOversizedOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{"wrong filename", "printf ID3 > other.mp3", ErrDownloadFailed},
		{"empty file", ": > audio.mp3", ErrDownloadFailed},
		{"symlink", "ln -s /etc/hosts audio.mp3", ErrDownloadFailed},
		{"oversized", "dd if=/dev/zero of=audio.mp3 bs=1048576 seek=21 count=0", ErrTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := New(testExecutable(t, tc.body)).Download(context.Background(), testYouTubeURL)
			if !errors.Is(err, tc.want) || data != nil {
				t.Fatalf("Download() returned %d bytes, %v, want no output and %v", len(data), err, tc.want)
			}
		})
	}
}

func TestDownloadPropagatesCallerCancellationDuringConversion(t *testing.T) {
	path := testExecutable(t, `
: > "$0.started"
sleep 30 &
wait
`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		_, err := New(path).Download(ctx, testYouTubeURL)
		result <- err
	}()

	for deadline := time.Now().Add(testProcessWait); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path + ".started"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("converter did not start")
		}
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Download() error = %v, want context.Canceled", err)
		}
	case <-time.After(testProcessWait):
		t.Fatal("Download() did not stop after cancellation")
	}
}

func TestResolveSoundCloudRedirect(t *testing.T) {
	for _, tc := range []struct {
		name     string
		location string
		want     string
		wantErr  error
	}{
		{"track", "https://soundcloud.com/artist/track?si=ignored", "https://soundcloud.com/artist/track", nil},
		{"outside soundcloud", "https://other.example", "", ErrInvalidURL},
		{"playlist", "https://soundcloud.com/artist/sets/playlist", "", ErrInvalidURL},
		{"downgrade", "http://soundcloud.com/artist/track", "", ErrInvalidURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests int
			client := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					if got := req.URL.String(); got != "https://on.soundcloud.com/Example9" {
						t.Errorf("request to unexpected redirect destination %q", got)
						return nil, errors.New("unexpected request")
					}
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{tc.location}},
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				}),
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			}
			got, err := resolveSoundCloudURL(context.Background(), client, "https://on.soundcloud.com/Example9")
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("resolveSoundCloudURL() = %q, %v, want %q, %v", got, err, tc.want, tc.wantErr)
			}
			if requests != 1 {
				t.Fatalf("made %d requests, want exactly one", requests)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testExecutable(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake converter uses a Unix shell")
	}
	path := filepath.Join(t.TempDir(), "fake-yt-dlp")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
