// Command objectsync copies one private S3-compatible bucket to or from a
// local directory for the production backup and restore jobs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type objectStore interface {
	list(ctx context.Context, bucket string) ([]string, error)
	download(ctx context.Context, bucket, key string, destination io.Writer) error
	upload(ctx context.Context, bucket, key string, source io.Reader, size int64) error
	delete(ctx context.Context, bucket, key string) error
}

type minioStore struct {
	client *minio.Client
}

func (s *minioStore) list(ctx context.Context, bucket string) ([]string, error) {
	var (
		keys   []string
		failed bool
	)
	for object := range s.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
		if object.Err != nil {
			failed = true
			continue
		}
		keys = append(keys, object.Key)
	}
	if failed {
		return nil, errors.New("object sync: object listing failed")
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *minioStore) download(ctx context.Context, bucket, key string, destination io.Writer) error {
	object, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return errors.New("object sync: object download failed")
	}
	defer object.Close()
	if _, err := io.Copy(destination, object); err != nil {
		return errors.New("object sync: object download failed")
	}
	return nil
}

func (s *minioStore) upload(ctx context.Context, bucket, key string, source io.Reader, size int64) error {
	if _, err := s.client.PutObject(ctx, bucket, key, source, size, minio.PutObjectOptions{}); err != nil {
		return errors.New("object sync: object upload failed")
	}
	return nil
}

func (s *minioStore) delete(ctx context.Context, bucket, key string) error {
	if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return errors.New("object sync: object deletion failed")
	}
	return nil
}

type syncConfig struct {
	endpoint     string
	secure       bool
	accessKey    string
	secretKey    string
	bucket       string
	region       string
	bucketLookup minio.BucketLookupType
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("object sync: backup or restore command is required")
	}

	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	directory := flags.String("directory", "", "local backup or restore directory")
	deleteExtraneous := flags.Bool("delete", false, "delete remote objects missing from the restore directory")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || strings.TrimSpace(*directory) == "" {
		return errors.New("object sync: invalid command arguments")
	}

	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	store, err := newMinioStore(cfg)
	if err != nil {
		return err
	}

	var count int
	switch command {
	case "backup":
		if *deleteExtraneous {
			return errors.New("object sync: delete is only valid for restore")
		}
		count, err = backupObjects(ctx, store, cfg.bucket, *directory)
	case "restore":
		count, err = restoreObjects(ctx, store, cfg.bucket, *directory, *deleteExtraneous)
	default:
		return errors.New("object sync: unsupported command")
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "synced_objects=%d\n", count)
	return nil
}

func loadConfig(getenv func(string) string) (syncConfig, error) {
	endpoint, secure, err := normalizeEndpoint(getenv("S3_ENDPOINT"), getenv("S3_USE_SSL"))
	if err != nil {
		return syncConfig{}, err
	}

	accessKey := firstNonempty(getenv("S3_ACCESS_KEY"), getenv("AWS_ACCESS_KEY_ID"))
	secretKey := firstNonempty(getenv("S3_SECRET_KEY"), getenv("AWS_SECRET_ACCESS_KEY"))
	bucket := strings.TrimSpace(getenv("S3_BUCKET"))
	if accessKey == "" || secretKey == "" {
		return syncConfig{}, errors.New("object sync: S3 credentials are required")
	}
	if bucket == "" {
		return syncConfig{}, errors.New("object sync: S3_BUCKET is required")
	}

	style := strings.ToLower(strings.TrimSpace(getenv("S3_ADDRESSING_STYLE")))
	if style == "" {
		style = "path"
	}
	var lookup minio.BucketLookupType
	switch style {
	case "path":
		lookup = minio.BucketLookupPath
	case "auto":
		lookup = minio.BucketLookupAuto
	case "virtual-hosted", "virtual", "dns":
		lookup = minio.BucketLookupDNS
	default:
		return syncConfig{}, errors.New("object sync: invalid S3_ADDRESSING_STYLE")
	}

	region := strings.TrimSpace(getenv("S3_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	return syncConfig{
		endpoint:     endpoint,
		secure:       secure,
		accessKey:    accessKey,
		secretKey:    secretKey,
		bucket:       bucket,
		region:       region,
		bucketLookup: lookup,
	}, nil
}

func normalizeEndpoint(rawEndpoint, rawUseSSL string) (string, bool, error) {
	endpoint := strings.TrimSpace(rawEndpoint)
	if endpoint == "" {
		return "", false, errors.New("object sync: S3_ENDPOINT is required")
	}
	secure, err := parseOptionalBool("S3_USE_SSL", rawUseSSL, false)
	if err != nil {
		return "", false, err
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		parsed, parseErr := url.Parse(endpoint)
		if parseErr != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", false, errors.New("object sync: invalid S3_ENDPOINT")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", false, errors.New("object sync: invalid S3_ENDPOINT scheme")
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return "", false, errors.New("object sync: S3_ENDPOINT must not include a path")
		}
		return parsed.Host, parsed.Scheme == "https", nil
	}

	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" || strings.ContainsAny(endpoint, "/?#@") || strings.ContainsAny(endpoint, "\r\n\t ") {
		return "", false, errors.New("object sync: invalid S3_ENDPOINT")
	}
	return endpoint, secure, nil
}

func parseOptionalBool(name, raw string, defaultValue bool) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("object sync: %s must be a boolean", name)
	}
	return value, nil
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func newMinioStore(cfg syncConfig) (*minioStore, error) {
	client, err := minio.New(cfg.endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.accessKey, cfg.secretKey, ""),
		Secure:       cfg.secure,
		Region:       cfg.region,
		BucketLookup: cfg.bucketLookup,
	})
	if err != nil {
		return nil, errors.New("object sync: unable to initialize S3 client")
	}
	return &minioStore{client: client}, nil
}

func backupObjects(ctx context.Context, store objectStore, bucket, destination string) (int, error) {
	rootPath, err := prepareDirectory(destination)
	if err != nil {
		return 0, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return 0, errors.New("object sync: unable to open backup directory")
	}
	defer root.Close()

	keys, err := store.list(ctx, bucket)
	if err != nil {
		return 0, errors.New("object sync: object listing failed")
	}

	count := 0
	for _, key := range keys {
		cleaned, directoryMarker, targetErr := cleanObjectKey(key)
		if targetErr != nil {
			return 0, targetErr
		}
		if directoryMarker {
			if err := ensureNoSymlinkComponents(root, cleaned); err != nil {
				return 0, err
			}
			if err := root.MkdirAll(cleaned, 0o700); err != nil {
				return 0, errors.New("object sync: unable to create backup directory")
			}
			if err := ensureNoSymlinkComponents(root, cleaned); err != nil {
				return 0, err
			}
			continue
		}
		parent := path.Dir(cleaned)
		if parent != "." {
			if err := ensureNoSymlinkComponents(root, parent); err != nil {
				return 0, err
			}
			if err := root.MkdirAll(parent, 0o700); err != nil {
				return 0, errors.New("object sync: unable to create backup directory")
			}
		}
		if err := ensureNoSymlinkComponents(root, cleaned); err != nil {
			return 0, err
		}
		destinationFile, err := root.OpenFile(cleaned, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return 0, errors.New("object sync: unable to create backup object")
		}
		downloadErr := store.download(ctx, bucket, key, destinationFile)
		closeErr := destinationFile.Close()
		if downloadErr != nil || closeErr != nil {
			_ = root.Remove(cleaned)
			return 0, errors.New("object sync: object download failed")
		}
		count++
	}
	return count, nil
}

type localObject struct {
	key  string
	size int64
}

func restoreObjects(ctx context.Context, store objectStore, bucket, source string, deleteExtraneous bool) (int, error) {
	rootPath, err := existingDirectory(source)
	if err != nil {
		return 0, err
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return 0, errors.New("object sync: unable to open restore directory")
	}
	defer root.Close()

	objects := make([]localObject, 0)
	err = fs.WalkDir(root.FS(), ".", func(localPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("object sync: unable to read restore directory")
		}
		if localPath == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("object sync: restore directory contains a symlink")
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			return errors.New("object sync: restore directory contains a non-regular file")
		}
		key := localPath
		if _, directoryMarker, keyErr := cleanObjectKey(key); keyErr != nil || directoryMarker {
			return errors.New("object sync: invalid restore path")
		}
		objects = append(objects, localObject{key: key, size: info.Size()})
		return nil
	})
	if err != nil {
		return 0, err
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].key < objects[j].key })

	localKeys := make(map[string]struct{}, len(objects))
	for _, object := range objects {
		if err := ensureNoSymlinkComponents(root, object.key); err != nil {
			return 0, err
		}
		sourceFile, err := root.Open(object.key)
		if err != nil {
			return 0, errors.New("object sync: unable to open restore object")
		}
		uploadErr := store.upload(ctx, bucket, object.key, sourceFile, object.size)
		closeErr := sourceFile.Close()
		if uploadErr != nil || closeErr != nil {
			return 0, errors.New("object sync: object upload failed")
		}
		localKeys[object.key] = struct{}{}
	}

	if deleteExtraneous {
		remoteKeys, err := store.list(ctx, bucket)
		if err != nil {
			return 0, errors.New("object sync: object listing failed")
		}
		sort.Strings(remoteKeys)
		for _, key := range remoteKeys {
			if _, exists := localKeys[key]; exists {
				continue
			}
			if err := store.delete(ctx, bucket, key); err != nil {
				return 0, errors.New("object sync: object deletion failed")
			}
		}
	}
	return len(objects), nil
}

func prepareDirectory(directory string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(directory))
	if err != nil {
		return "", errors.New("object sync: invalid backup directory")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", errors.New("object sync: unable to create backup directory")
	}
	return validateDirectoryRoot(root)
}

func existingDirectory(directory string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(directory))
	if err != nil {
		return "", errors.New("object sync: invalid restore directory")
	}
	return validateDirectoryRoot(root)
}

func validateDirectoryRoot(root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("object sync: local directory is unavailable or unsafe")
	}
	return filepath.Clean(root), nil
}

func cleanObjectKey(key string) (string, bool, error) {
	if key == "" || strings.ContainsRune(key, '\x00') || strings.Contains(key, `\`) {
		return "", false, errors.New("object sync: unsafe object key rejected")
	}
	directoryMarker := strings.HasSuffix(key, "/")
	key = strings.TrimSuffix(key, "/")
	cleaned := path.Clean(key)
	if key == "" || cleaned != key || path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false, errors.New("object sync: unsafe object key rejected")
	}
	return cleaned, directoryMarker, nil
}

func ensureNoSymlinkComponents(root *os.Root, relative string) error {
	cleaned, directoryMarker, err := cleanObjectKey(relative)
	if err != nil || directoryMarker {
		return errors.New("object sync: unsafe local path rejected")
	}
	current := ""
	for _, component := range strings.Split(cleaned, "/") {
		current = path.Join(current, component)
		info, statErr := root.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return errors.New("object sync: unable to validate local path")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("object sync: symlink path rejected")
		}
	}
	return nil
}
