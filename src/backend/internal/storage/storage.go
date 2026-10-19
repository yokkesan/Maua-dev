package storage

import (
	"context"
	"io"
)

// Storage は画像ファイルの保存先を抽象化するインターフェースです。
type Storage interface {
	// Save は指定されたキーでファイルを保存します。
	Save(ctx context.Context, key string, file io.Reader) error

	// Delete は指定されたキーのファイルを削除します。
	Delete(ctx context.Context, key string) error
}
