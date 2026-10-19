package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalStorageSave(t *testing.T) {
	root := t.TempDir()

	s, err := NewLocalStorage(root)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	key := "brands/1/main/1_test.jpg"
	content := []byte("test image content")

	err = s.Save(
		context.Background(),
		key,
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	saved, err := os.ReadFile(
		filepath.Join(root, filepath.FromSlash(key)),
	)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	if !bytes.Equal(saved, content) {
		t.Fatalf("saved content does not match")
	}
}

func TestLocalStorageSaveDuplicate(t *testing.T) {
	s, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	key := "brands/1/main/test.jpg"

	if err := s.Save(
		context.Background(),
		key,
		bytes.NewReader([]byte("first")),
	); err != nil {
		t.Fatalf("first Save() error: %v", err)
	}

	if err := s.Save(
		context.Background(),
		key,
		bytes.NewReader([]byte("second")),
	); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestLocalStorageDelete(t *testing.T) {
	root := t.TempDir()

	s, err := NewLocalStorage(root)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	key := "shops/1/main/test.jpg"

	if err := s.Save(
		context.Background(),
		key,
		bytes.NewReader([]byte("test")),
	); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	path := filepath.Join(root, filepath.FromSlash(key))

	_, err = os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be deleted, got: %v", err)
	}

	// 削除済みファイルの再削除も成功する
	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete() on missing file error: %v", err)
	}
}

func TestLocalStorageInvalidKeys(t *testing.T) {
	s, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	keys := []string{
		"",
		".",
		"..",
		"../outside.jpg",
		"brands/../../outside.jpg",
		"/tmp/outside.jpg",
		`brands\outside.jpg`,
	}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			err := s.Save(
				context.Background(),
				key,
				bytes.NewReader([]byte("test")),
			)
			if err == nil {
				t.Fatalf("Save(%q) should fail", key)
			}
		})
	}
}

func TestLocalStorageCanceledContext(t *testing.T) {
	s, err := NewLocalStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = s.Save(
		ctx,
		"test.jpg",
		bytes.NewReader([]byte("test")),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

type failingReader struct{}

func (failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("simulated read failure")
}

func TestLocalStorageSaveFailureCleanup(t *testing.T) {
	root := t.TempDir()

	s, err := NewLocalStorage(root)
	if err != nil {
		t.Fatalf("NewLocalStorage() error: %v", err)
	}

	key := "brands/1/main/failing.jpg"

	err = s.Save(
		context.Background(),
		key,
		failingReader{},
	)

	if err == nil {
		t.Fatal("expected Save() error")
	}

	path := filepath.Join(root, filepath.FromSlash(key))

	_, err = os.Stat(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial file should be removed, got: %v", err)
	}
}
