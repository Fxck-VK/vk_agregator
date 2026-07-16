package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type fakeObjectStore struct {
	remote       []string
	downloaded   []string
	uploaded     []string
	deleted      []string
	downloadData map[string]string
	failUpload   string
}

func (s *fakeObjectStore) list(context.Context, string) ([]string, error) {
	return append([]string(nil), s.remote...), nil
}

func (s *fakeObjectStore) download(_ context.Context, _, key string, destination io.Writer) error {
	s.downloaded = append(s.downloaded, key)
	if _, err := io.WriteString(destination, s.downloadData[key]); err != nil {
		return err
	}
	return nil
}

func (s *fakeObjectStore) upload(_ context.Context, _, key string, source io.Reader, size int64) error {
	s.uploaded = append(s.uploaded, key)
	if key == s.failUpload {
		return errors.New("provider failure containing private-object-name")
	}
	data, err := io.ReadAll(io.LimitReader(source, size+1))
	if err != nil || int64(len(data)) != size {
		return errors.New("invalid source payload")
	}
	return nil
}

func (s *fakeObjectStore) delete(_ context.Context, _, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

func TestBackupObjectsRejectsUnsafeObjectKeys(t *testing.T) {
	t.Parallel()

	tests := []string{
		"../escape",
		"nested/../../escape",
		"/absolute",
		`windows\escape`,
		"nested//ambiguous",
	}
	for _, key := range tests {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			store := &fakeObjectStore{remote: []string{key}}
			_, err := backupObjects(context.Background(), store, "bucket", t.TempDir())
			if err == nil {
				t.Fatalf("backupObjects(%q) succeeded, want rejection", key)
			}
			if len(store.downloaded) != 0 {
				t.Fatalf("unsafe key %q reached download: %v", key, store.downloaded)
			}
			if strings.Contains(err.Error(), key) {
				t.Fatalf("error exposed an object key: %v", err)
			}
		})
	}
}

func TestBackupObjectsDownloadsNestedObjects(t *testing.T) {
	t.Parallel()

	destination := t.TempDir()
	store := &fakeObjectStore{
		remote: []string{"one.txt", "nested/two.bin"},
		downloadData: map[string]string{
			"one.txt":        "one",
			"nested/two.bin": "two",
		},
	}

	count, err := backupObjects(context.Background(), store, "bucket", destination)
	if err != nil {
		t.Fatalf("backupObjects() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("backupObjects() count = %d, want 2", count)
	}
	for relative, want := range map[string]string{"one.txt": "one", "nested/two.bin": "two"} {
		got, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", relative, got, want)
		}
	}
}

func TestBackupObjectsRejectsSymlinkDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not available in every Windows test environment")
	}

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "nested")); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	store := &fakeObjectStore{remote: []string{"nested/object.txt"}}

	if _, err := backupObjects(context.Background(), store, "bucket", root); err == nil {
		t.Fatal("backupObjects() succeeded through a symlink")
	}
	if len(store.downloaded) != 0 {
		t.Fatalf("symlink destination reached download: %v", store.downloaded)
	}
}

func TestRestoreObjectsUploadsBeforeDeletingExtraneousObjects(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	writeTestFile(t, source, "one.txt", "one")
	writeTestFile(t, source, "nested/two.bin", "two")
	store := &fakeObjectStore{remote: []string{"one.txt", "stale.bin"}}

	count, err := restoreObjects(context.Background(), store, "bucket", source, true)
	if err != nil {
		t.Fatalf("restoreObjects() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("restoreObjects() count = %d, want 2", count)
	}
	if !reflect.DeepEqual(store.uploaded, []string{"nested/two.bin", "one.txt"}) {
		t.Fatalf("uploaded = %v", store.uploaded)
	}
	if !reflect.DeepEqual(store.deleted, []string{"stale.bin"}) {
		t.Fatalf("deleted = %v", store.deleted)
	}
}

func TestRestoreObjectsDoesNotDeleteAfterUploadFailure(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	writeTestFile(t, source, "one.txt", "one")
	writeTestFile(t, source, "two.txt", "two")
	store := &fakeObjectStore{
		remote:     []string{"stale.bin"},
		failUpload: "two.txt",
	}

	_, err := restoreObjects(context.Background(), store, "bucket", source, true)
	if err == nil {
		t.Fatal("restoreObjects() succeeded despite upload failure")
	}
	if len(store.deleted) != 0 {
		t.Fatalf("restore deleted objects after failed upload: %v", store.deleted)
	}
	if strings.Contains(err.Error(), "private-object-name") || strings.Contains(err.Error(), "two.txt") {
		t.Fatalf("restore error exposed provider details or an object key: %v", err)
	}
}

func TestRestoreObjectsRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not available in every Windows test environment")
	}

	source := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "linked.txt")); err != nil {
		t.Skipf("create symlink: %v", err)
	}
	store := &fakeObjectStore{}

	if _, err := restoreObjects(context.Background(), store, "bucket", source, false); err == nil {
		t.Fatal("restoreObjects() accepted a symlink")
	}
	if len(store.uploaded) != 0 {
		t.Fatalf("symlink reached upload: %v", store.uploaded)
	}
}

func TestLoadConfigRejectsMalformedEndpointWithoutExposingCredentials(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"S3_ENDPOINT":   "https://storage.invalid/private/path",
		"S3_ACCESS_KEY": "sensitive-access",
		"S3_SECRET_KEY": "sensitive-secret",
		"S3_BUCKET":     "private-bucket",
	}
	_, err := loadConfig(func(key string) string { return values[key] })
	if err == nil {
		t.Fatal("loadConfig() accepted an endpoint path")
	}
	for _, secret := range []string{"sensitive-access", "sensitive-secret", "private-bucket"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("loadConfig() error exposed sensitive config: %v", err)
		}
	}
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
