package brand

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"

	"backend/internal/storage"

	"github.com/jackc/pgx/v5"
)

// saveBrandMainImages は、画像ファイルを保存し、
// 同じDBトランザクションに画像情報を登録する。
//
// この関数はCommitを行わない。
// 呼び出し元がトランザクションの確定を管理する。
func saveBrandMainImages(
	ctx context.Context,
	tx pgx.Tx,
	brandID int64,
	files []*multipart.FileHeader,
	imageService *storage.MainImageService,
) ([]storage.SavedMainImage, error) {
	if tx == nil {
		return nil, errors.New("transaction is required")
	}

	if brandID <= 0 {
		return nil, errors.New("invalid brand ID")
	}

	if imageService == nil {
		return nil, errors.New("image service is required")
	}

	// 画像2枚を保存する。
	savedImages, err := imageService.Save(
		ctx,
		"brands",
		brandID,
		files,
	)
	if err != nil {
		return nil, fmt.Errorf("save brand main images: %w", err)
	}

	// 保存した画像情報を、既存のDBトランザクションへ登録する。
	for _, image := range savedImages {
		_, err := tx.Exec(
			ctx,
			`INSERT INTO brand_images (
				brand_id,
				slot_number,
				storage_key
			) VALUES ($1, $2, $3)`,
			brandID,
			image.SlotNumber,
			image.StorageKey,
		)

		if err != nil {
			cleanupErr := imageService.Cleanup(savedImages)

			return nil, errors.Join(
				fmt.Errorf("insert brand image: %w", err),
				cleanupErr,
			)
		}
	}

	return savedImages, nil
}
