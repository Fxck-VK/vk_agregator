package mediaprobe

import (
	"bytes"
	"context"
	"encoding/binary"
	"os/exec"
	"testing"
	"time"
)

func TestFFProbeAudioReadsDurationFromRealPCMBytes(t *testing.T) {
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe is not installed")
	}
	// One second of synthetic silence, mono signed PCM, 8 kHz / 16 bit.
	data := make([]byte, 44+16000)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], 8000)
	binary.LittleEndian.PutUint32(data[28:], 16000)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], 16000)
	metadata, err := NewFFProbe(Config{FFProbePath: path}).ProbeAudio(context.Background(), data, int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.DurationMS != 1000 || metadata.Container != "wav" {
		t.Fatalf("wrong real audio facts: %+v", metadata)
	}
	encoder, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Log("MP3/AAC checks skipped: ffmpeg not installed")
		return
	}
	for _, test := range []struct{ name, codec, format, mime string }{{"mp3", "libmp3lame", "mp3", "audio/mpeg"}, {"aac", "aac", "adts", "audio/aac"}} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, encoder, "-v", "error", "-f", "wav", "-i", "pipe:0", "-map", "0:a:0", "-c:a", test.codec, "-f", test.format, "pipe:1")
			cmd.Stdin = bytes.NewReader(data)
			encoded, err := cmd.Output()
			if err != nil {
				t.Fatalf("encode synthetic input: %v", err)
			}
			mime, ok := SniffMusicInputMIME(encoded)
			if !ok || mime != test.mime {
				t.Fatalf("incorrect audio detection: %s", mime)
			}
			metadata, err := NewFFProbe(Config{FFProbePath: path}).ProbeAudio(ctx, encoded, int64(len(encoded)))
			if err != nil {
				t.Fatal(err)
			}
			if metadata.DurationMS < 900 || metadata.DurationMS > 1300 {
				t.Fatalf("wrong decoded duration: %d", metadata.DurationMS)
			}
		})
	}
}
