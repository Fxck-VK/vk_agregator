package audiodownload

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	metadataName     = "source.json"
	metadataTemplate = `after_move:{"title":%(title|null)j,"artist":%(artist|null)j,"uploader":%(uploader|null)j}`
	maxMetadataBytes = 32 << 10
	filenameSuffix   = " epifera.com.mp3"
	maxFilenameBytes = 240
)

// Result contains the converted audio and its source-derived download filename.
type Result struct {
	Data     []byte
	Filename string
}

// NormalizeFilename bounds a download filename while preserving Unicode titles.
func NormalizeFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if strings.HasSuffix(strings.ToLower(filename), ".mp3") {
		filename = filename[:len(filename)-4]
	}
	if strings.HasSuffix(strings.ToLower(filename), " epifera.com") {
		filename = filename[:len(filename)-len(" epifera.com")]
	}
	filename = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return ' '
		}
		return r
	}, filename)
	filename = strings.Trim(strings.Join(strings.Fields(filename), " "), ". ")
	for len(filename) > maxFilenameBytes-len(filenameSuffix) {
		_, size := utf8.DecodeLastRuneInString(filename)
		filename = filename[:len(filename)-size]
	}
	filename = strings.TrimRight(filename, ". ")
	if filename == "" {
		filename = "audio"
	}
	return filename + filenameSuffix
}

func sourceFilename(dir, canonical string) string {
	path := filepath.Join(dir, metadataName)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxMetadataBytes {
		return NormalizeFilename("")
	}
	f, err := os.Open(path)
	if err != nil {
		return NormalizeFilename("")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxMetadataBytes+1))
	if err != nil || len(data) > maxMetadataBytes {
		return NormalizeFilename("")
	}
	var metadata struct {
		Title    string `json:"title"`
		Artist   string `json:"artist"`
		Uploader string `json:"uploader"`
	}
	if json.Unmarshal(data, &metadata) != nil {
		return NormalizeFilename("")
	}
	title := strings.TrimSpace(metadata.Title)
	if title == "" {
		title = "audio"
	}
	if strings.HasPrefix(canonical, "https://soundcloud.com/") {
		artist := strings.TrimSpace(metadata.Artist)
		if artist == "" {
			artist = strings.TrimSpace(metadata.Uploader)
		}
		if artist != "" {
			title += " - " + artist
		}
	}
	return NormalizeFilename(title)
}
