package s3

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type countingReader struct {
	io.Reader
	read int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	return n, err
}

func TestBoundedObjectReadDoesNotDownloadOversizedBody(t *testing.T) {
	source := &countingReader{Reader: strings.NewReader(strings.Repeat("x", 1000))}
	data, err := readObjectWithinLimit(source, 4)
	if err == nil || len(data) != 0 || source.read != 5 {
		t.Fatalf("bytes=%d error=%v read=%d", len(data), err, source.read)
	}
}

func TestBoundedObjectReadAcceptsExactLimit(t *testing.T) {
	data, err := readObjectWithinLimit(strings.NewReader("webp"), 4)
	if err != nil || string(data) != "webp" {
		t.Fatalf("data=%q error=%v", data, err)
	}
	for _, limit := range []int64{0, -1, 1<<63 - 1} {
		if _, err := readObjectWithinLimit(strings.NewReader("x"), limit); err == nil {
			t.Fatalf("accepted invalid limit %d", limit)
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestBoundedObjectReadRejectsEmptyOrFailedReads(t *testing.T) {
	for _, reader := range []io.Reader{strings.NewReader(""), failingReader{}} {
		if _, err := readObjectWithinLimit(reader, 4); err == nil {
			t.Fatal("accepted empty/failed read")
		}
	}
}
