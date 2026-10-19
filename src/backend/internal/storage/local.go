package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage はローカルファイルシステムへの保存処理です。
type LocalStorage struct {
	rootDir string
}

// NewLocalStorage は保存先のルートディレクトリを設定します。
func NewLocalStorage(rootDir string) (*LocalStorage, error) {
	if strings.TrimSpace(rootDir) == "" {
		return nil, errors.New("storage root directory is required")
	}

	absolutePath, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve storage directory: %w", err)
	}

	if err := os.MkdirAll(absolutePath, 0755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	return &LocalStorage{
		rootDir: absolutePath,
	}, nil
}

// resolvePath は保存キーからローカルのファイルパスを取得します。
func (s *LocalStorage) resolvePath(key string) (string, error) {
	if key == "" {
		return "", errors.New("storage key is required")
	}

	if filepath.IsAbs(key) {
		return "", errors.New("absolute storage key is not allowed")
	}

	if strings.Contains(key, "\\") {
		return "", errors.New("backslash is not allowed in storage key")
	}

	cleanKey := filepath.Clean(key)

	if cleanKey == "." ||
		cleanKey == ".." ||
		strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid storage key")
	}

	return filepath.Join(s.rootDir, cleanKey), nil
}

// Save は指定キーにファイルを保存します。
func (s *LocalStorage) Save(
	ctx context.Context,
	key string,
	file io.Reader,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.resolvePath(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create image file: %w", err)
	}

	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(path)
		return fmt.Errorf("write image file: %w", err)
	}

	if err := out.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("close image file: %w", err)
	}

	return nil
}

// Delete は指定キーのファイルを削除します。
// ファイルが存在しない場合は正常終了します。
func (s *LocalStorage) Delete(
	ctx context.Context,
	key string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path, err := s.resolvePath(key)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete image file: %w", err)
	}

	return nil
}
