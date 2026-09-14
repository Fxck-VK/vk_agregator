package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRejectsUnknownFieldsAndMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contract.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"suports_images":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{path, path + "-missing"} {
		var out, errors bytes.Buffer
		if code := run([]string{"-contract", file}, &out, &errors); code == 0 || errors.Len() == 0 {
			t.Fatalf("invalid contract passed: %d %s", code, errors.String())
		}
	}
}

func TestRegistryCheckDistinguishesLegacyFromVerified(t *testing.T) {
	var out, errors bytes.Buffer
	if code := run(nil, &out, &errors); code != 0 {
		t.Fatalf("%d: %s", code, errors.String())
	}
	if !strings.Contains(out.String(), "legacy-unverified") {
		t.Fatal("migration record presented as verified")
	}
}
