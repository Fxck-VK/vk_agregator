package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestChecksumNormalizesLineEndings(t *testing.T) {
	lf := []byte("CREATE TABLE example (id BIGINT);\nSELECT 1;\n")
	crlf := []byte("CREATE TABLE example (id BIGINT);\r\nSELECT 1;\r\n")

	if got, want := checksum(crlf), checksum(lf); got != want {
		t.Fatalf("checksum differs by line endings: got %s, want %s", got, want)
	}
}

func TestChecksumMatchesLegacyCRLF(t *testing.T) {
	lf := []byte("CREATE TABLE example (id BIGINT);\nSELECT 1;\n")
	crlf := bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))
	legacy := fmt.Sprintf("%x", sha256.Sum256(crlf))

	if !checksumMatches(legacy, lf) {
		t.Fatalf("legacy CRLF checksum %s was rejected", legacy)
	}
}

func TestChecksumMatchesRejectsSQLDrift(t *testing.T) {
	recorded := checksum([]byte("SELECT 1;\n"))

	if checksumMatches(recorded, []byte("SELECT 2;\n")) {
		t.Fatal("checksum accepted changed SQL")
	}
}
