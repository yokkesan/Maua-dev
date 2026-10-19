package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"path"
	"time"
)

// SavedMainImage は保存済み画像の情報です。
type SavedMainImage struct {
	SlotNumber int
	StorageKey string
}

// MainImageService はメイン画像の保存処理を担当します。
type MainImageService struct {
	storage Storage
}

// NewMainImageService は画像保存サービスを生成します。
func NewMainImageService(s Storage) (*MainImageService, error) {
	if s == nil {
		return nil, errors.New("storage is required")
	}

	return &MainImageService{
		storage: s,
	}, nil
}

// Save はメイン画像2枚を保存します。
//
// ownerType は "brands" または "shops" を指定します。
// ownerID は登録済みのブランド・ショップIDです。
//
// 途中で失敗した場合は保存済みファイルの回収を試みます。
func (s *MainImageService) Save(
	ctx context.Context,
	ownerType string,
	ownerID int64,
	files []*multipart.FileHeader,
) ([]SavedMainImage, error) {
	if ownerType != "brands" && ownerType != "shops" {
		return nil, errors.New("invalid image owner type")
	}

	if ownerID <= 0 {
		return nil, errors.New("invalid image owner ID")
	}

	if err := ValidateMainImages(files); err != nil {
		return nil, err
	}

	images := make([]SavedMainImage, 0, RequiredMainImageCount)

	for index, header := range files {
		file, err := header.Open()
		if err != nil {
			cleanupErr := s.Cleanup(images)
			return nil, errors.Join(
				fmt.Errorf("open image: %w", err),
				cleanupErr,
			)
		}

		validated, validateErr := ValidateImage(file)
		closeErr := file.Close()

		if validateErr != nil || closeErr != nil {
			cleanupErr := s.Cleanup(images)
			return nil, errors.Join(
				validateErr,
				closeErr,
				cleanupErr,
			)
		}

		randomName, err := generateImageName()
		if err != nil {
			cleanupErr := s.Cleanup(images)
			return nil, errors.Join(err, cleanupErr)
		}

		slotNumber := index + 1

		key := path.Join(
			ownerType,
			fmt.Sprintf("%d", ownerID),
			"main",
			fmt.Sprintf(
				"%d_%s%s",
				slotNumber,
				randomName,
				validated.Extension,
			),
		)

		if err := s.storage.Save(
			ctx,
			key,
			bytes.NewReader(validated.Data),
		); err != nil {
			// Saveが部分的にファイルを作成した可能性も考慮する。
			cleanupTargets := append(
				images,
				SavedMainImage{
					SlotNumber: slotNumber,
					StorageKey: key,
				},
			)

			cleanupErr := s.Cleanup(cleanupTargets)

			return nil, errors.Join(
				fmt.Errorf("save main image: %w", err),
				cleanupErr,
			)
		}

		images = append(images, SavedMainImage{
			SlotNumber: slotNumber,
			StorageKey: key,
		})
	}

	return images, nil
}

// Cleanup は保存済み画像を削除します。
//
// リクエストがキャンセルされていても回収できるように、
// リクエストのContextには依存しません。
func (s *MainImageService) Cleanup(images []SavedMainImage) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	var cleanupErr error

	for _, img := range images {
		if err := s.storage.Delete(ctx, img.StorageKey); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf(
					"cleanup image %s: %w",
					img.StorageKey,
					err,
				),
			)
		}
	}

	return cleanupErr
}

// generateImageName は推測困難なランダム文字列を生成します。
func generateImageName() (string, error) {
	randomBytes := make([]byte, 20)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate image name: %w", err)
	}

	return hex.EncodeToString(randomBytes), nil
}
