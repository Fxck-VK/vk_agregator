package audiodownload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDownloadWithMetadataNamesSourcesAndKeepsAudioAPI(t *testing.T) {
	for _, tc := range []struct {
		name, source, metadata, want string
	}{
		{"youtube", "https://youtu.be/abcdefghijk", `{"title":"Название ролика","artist":"не нужен","uploader":"Канал"}`, "Название ролика epifera.com.mp3"},
		{"soundcloud artist", "https://soundcloud.com/artist/track", `{"title":"Название трека","artist":"Исполнитель","uploader":"Загрузивший"}`, "Название трека - Исполнитель epifera.com.mp3"},
		{"soundcloud uploader", "https://soundcloud.com/artist/track", `{"title":"Track","artist":null,"uploader":"Artist"}`, "Track - Artist epifera.com.mp3"},
		{"soundcloud no artist", "https://soundcloud.com/artist/track", `{"title":"Track","artist":null,"uploader":null}`, "Track epifera.com.mp3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := testExecutable(t, `
case " $* " in
  *' --print-to-file after_move:{"title":%(title|null)j,"artist":%(artist|null)j,"uploader":%(uploader|null)j} source.json '*) ;;
  *) exit 1 ;;
esac
printf 'ID3example' > audio.mp3
cat > source.json <<'METADATA'
`+tc.metadata+`
METADATA
`)
			downloader := New(path)
			result, err := downloader.DownloadWithMetadata(context.Background(), tc.source)
			if err != nil || string(result.Data) != "ID3example" || result.Filename != tc.want {
				t.Fatalf("DownloadWithMetadata() = %+v, %v, want filename %q", result, err, tc.want)
			}
			data, err := downloader.Download(context.Background(), tc.source)
			if err != nil || string(data) != "ID3example" {
				t.Fatalf("Download() returned %q, %v", data, err)
			}
		})
	}
}

func TestSourceFilenameFallsBackForInvalidMetadata(t *testing.T) {
	for _, body := range []string{"", "{", `{"title":7}`, strings.Repeat(" ", maxMetadataBytes+1)} {
		t.Run("metadata", func(t *testing.T) {
			dir := t.TempDir()
			if body != "" {
				if err := os.WriteFile(filepath.Join(dir, metadataName), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if got := sourceFilename(dir, testYouTubeURL); got != "audio epifera.com.mp3" {
				t.Fatalf("fallback filename = %q", got)
			}
		})
	}
}

func TestNormalizeFilenamePreservesSuffixAndSafeUnicode(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"Трек - Исполнитель epifera.com.mp3", "Трек - Исполнитель epifera.com.mp3"},
		{" Title  with\tspaces ", "Title with spaces epifera.com.mp3"},
		{"../../Title\\file:part?*.mp3", "Title file part epifera.com.mp3"},
		{"Title\r\nInjected\x00name\u202efile", "Title Injectednamefile epifera.com.mp3"},
		{"", "audio epifera.com.mp3"},
		{"... /\\", "audio epifera.com.mp3"},
	} {
		if got := NormalizeFilename(tc.input); got != tc.want {
			t.Errorf("NormalizeFilename(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
	long := NormalizeFilename(strings.Repeat("Название", 100))
	if len(long) > maxFilenameBytes || !utf8.ValidString(long) || !strings.HasSuffix(long, filenameSuffix) {
		t.Fatalf("invalid long filename length=%d, value=%q", len(long), long)
	}
}
