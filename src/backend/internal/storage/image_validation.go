package storage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"

	_ "image/gif"
)

const (
	// MaxImageSize は画像1枚あたりの最大容量（5MB）です。
	MaxImageSize int64 = 5_000_000

	// RequiredMainImageCount は登録時に必要なメイン画像の枚数です。
	RequiredMainImageCount = 2

	// MaxImageWidth / MaxImageHeight は画像の最大寸法です。
	MaxImageWidth  = 10000
	MaxImageHeight = 10000
)

var (
	ErrInvalidImage      = errors.New("invalid image")
	ErrImageTooLarge     = errors.New("image too large")
	ErrInvalidImageCount = errors.New("invalid image count")
)

// ValidatedImage は検証済みの画像データです。
type ValidatedImage struct {
	Data        []byte
	ContentType string
	Extension   string
	Width       int
	Height      int
}

// ValidateMainImages はメイン画像が2枚揃っていることを検証します。
func ValidateMainImages(files []*multipart.FileHeader) error {
	if len(files) != RequiredMainImageCount {
		return fmt.Errorf(
			"%w: expected %d images, got %d",
			ErrInvalidImageCount,
			RequiredMainImageCount,
			len(files),
		)
	}

	for _, file := range files {
		if file == nil {
			return ErrInvalidImage
		}

		if file.Size <= 0 {
			return ErrInvalidImage
		}

		if file.Size > MaxImageSize {
			return ErrImageTooLarge
		}
	}

	return nil
}

// ValidateImage は画像の実データを検証します。
// ファイル名やContent-Typeヘッダーだけでは判定しません。
func ValidateImage(file multipart.File) (*ValidatedImage, error) {
	if file == nil {
		return nil, ErrInvalidImage
	}

	data, err := io.ReadAll(io.LimitReader(file, MaxImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}

	if len(data) == 0 {
		return nil, ErrInvalidImage
	}

	if int64(len(data)) > MaxImageSize {
		return nil, ErrImageTooLarge
	}

	// ファイルの実データから形式を判定します。
	contentType := ""
	extension := ""

	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		contentType = "image/jpeg"
		extension = ".jpg"

	case bytes.HasPrefix(data, []byte{
		0x89, 0x50, 0x4E, 0x47,
		0x0D, 0x0A, 0x1A, 0x0A,
	}):
		contentType = "image/png"
		extension = ".png"

	default:
		return nil, ErrInvalidImage
	}

	// 画像として解析可能か確認します。
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidImage
	}

	if format != "jpeg" && format != "png" {
		return nil, ErrInvalidImage
	}

	if config.Width <= 0 ||
		config.Height <= 0 ||
		config.Width > MaxImageWidth ||
		config.Height > MaxImageHeight {
		return nil, ErrInvalidImage
	}

	// 画像全体をデコードして、破損ファイルを拒否します。
	switch format {
	case "jpeg":
		_, err = jpeg.Decode(bytes.NewReader(data))
	case "png":
		_, err = png.Decode(bytes.NewReader(data))
	}

	if err != nil {
		return nil, ErrInvalidImage
	}

	return &ValidatedImage{
		Data:        data,
		ContentType: contentType,
		Extension:   extension,
		Width:       config.Width,
		Height:      config.Height,
	}, nil
}
