package audiodownload

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	downloadTimeout = 2 * time.Minute
	maxAudioBytes   = 20 << 20
	maxWorkingBytes = 56 << 20
	outputName      = "audio.mp3"
)

var (
	// ErrUnavailable indicates the configured downloader cannot be started.
	ErrUnavailable = errors.New("audiodownload: downloader unavailable")
	// ErrDownloadFailed covers inaccessible media and conversion failures.
	ErrDownloadFailed = errors.New("audiodownload: could not prepare audio")
	// ErrTimeout indicates the operation exceeded its own time limit.
	ErrTimeout = errors.New("audiodownload: time limit exceeded")
	// ErrTooLarge indicates an audio file exceeded a size limit.
	ErrTooLarge = errors.New("audiodownload: size limit exceeded")
)

// Downloader uses yt-dlp and FFmpeg to extract one MP3 at a time.
type Downloader struct {
	path string
	slot chan struct{}
}

// New configures the yt-dlp executable. A blank path uses yt-dlp from PATH.
func New(path string) *Downloader {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "yt-dlp"
	} else if filepath.Base(path) != path && !filepath.IsAbs(path) {
		// The converter runs in a temporary directory, independent of where
		// the executable was configured.
		if absolute, err := filepath.Abs(path); err == nil {
			path = absolute
		}
	}
	return &Downloader{path: path, slot: make(chan struct{}, 1)}
}

// Download returns MP3 bytes. Errors do not include URLs, filesystem paths or
// third-party tool output.
func (d *Downloader) Download(ctx context.Context, sourceURL string) ([]byte, error) {
	result, err := d.DownloadWithMetadata(ctx, sourceURL)
	return result.Data, err
}

// DownloadWithMetadata returns MP3 bytes and a safe filename from the source.
func (d *Downloader) DownloadWithMetadata(ctx context.Context, sourceURL string) (Result, error) {
	canonical, err := NormalizeURL(sourceURL)
	if err != nil {
		return Result{}, err
	}

	downloadCtx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	select {
	case d.slot <- struct{}{}:
		defer func() { <-d.slot }()
	case <-downloadCtx.Done():
		return Result{}, downloadContextError(ctx, downloadCtx)
	}

	if isSoundCloudShareURL(canonical) {
		canonical, err = resolveSoundCloudURL(downloadCtx, soundCloudRedirectClient, canonical)
		if downloadCtx.Err() != nil {
			return Result{}, downloadContextError(ctx, downloadCtx)
		}
		if err != nil {
			return Result{}, err
		}
	}

	workDir, err := os.MkdirTemp("", "epifera-mp3-")
	if err != nil {
		return Result{}, ErrDownloadFailed
	}
	defer os.RemoveAll(workDir)

	err = d.run(downloadCtx, workDir, canonical)
	if downloadCtx.Err() != nil {
		return Result{}, downloadContextError(ctx, downloadCtx)
	}
	if err != nil {
		return Result{}, err
	}
	data, err := readAudio(filepath.Join(workDir, outputName))
	if err != nil {
		return Result{}, err
	}
	return Result{Data: data, Filename: sourceFilename(workDir, canonical)}, nil
}

func downloadContextError(parent, download context.Context) error {
	if err := parent.Err(); err != nil {
		return err
	}
	if errors.Is(download.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return download.Err()
}

func (d *Downloader) run(ctx context.Context, dir, canonical string) error {
	extractor := "youtube"
	if strings.HasPrefix(canonical, "https://soundcloud.com/") {
		extractor = "soundcloud"
	}
	args := []string{
		"--ignore-config", "--no-plugin-dirs", "--no-cache-dir", "--abort-on-error", "--no-playlist",
		"--use-extractors", extractor,
		"--js-runtimes", "node",
		"--format", "bestaudio[filesize<=?20MiB][filesize_approx<=?20MiB]/best[filesize<=?20MiB][filesize_approx<=?20MiB]",
		"--format-sort", "abr:128",
		"--match-filters", "duration <= 1200 & !is_live",
		"--socket-timeout", "15", "--retries", "1", "--fragment-retries", "1",
		"--max-filesize", strconv.Itoa(maxAudioBytes), "--limit-rate", "4M",
		"--extract-audio", "--audio-format", "mp3", "--audio-quality", "128K",
		"--postprocessor-args", "ExtractAudio+ffmpeg_o:-fs " + strconv.Itoa(maxAudioBytes+1),
		"--print-to-file", metadataTemplate, metadataName,
		"--output", "audio.%(ext)s", "--", canonical,
	}

	processCtx, stopProcess := context.WithCancel(ctx)
	defer stopProcess()
	cmd := exec.CommandContext(processCtx, d.path, args...) // #nosec G204 -- configured executable; no shell; URL is canonicalized.
	cmd.Dir = dir
	// Keep worker credentials and the operator's HOME out of child processes.
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + dir,
		"TMPDIR=" + dir,
		"TMP=" + dir,
		"TEMP=" + dir,
	}
	setProcessCancellation(cmd)
	if err := cmd.Start(); err != nil {
		return ErrUnavailable
	}

	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-finished:
			if err != nil {
				return ErrDownloadFailed
			}
			return nil
		case <-ticker.C:
			if err := checkWorkingSize(dir); err != nil {
				stopProcess()
				<-finished
				return err
			}
		}
	}
}

func checkWorkingSize(dir string) error {
	var total int64
	return filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		// The tools may move/delete temporary files while the directory is read.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return ErrDownloadFailed
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return ErrDownloadFailed
		}
		total += info.Size()
		if info.Size() > maxAudioBytes || total > maxWorkingBytes {
			return ErrTooLarge
		}
		return nil
	})
}

func readAudio(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, ErrDownloadFailed
	}
	if info.Size() > maxAudioBytes {
		return nil, ErrTooLarge
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrDownloadFailed
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxAudioBytes+1))
	if err != nil || len(data) == 0 {
		return nil, ErrDownloadFailed
	}
	if len(data) > maxAudioBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}
