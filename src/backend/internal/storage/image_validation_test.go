package storage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func createTestImage(t *testing.T, format string) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(5, 5, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	var err error

	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
	case "png":
		err = png.Encode(&buf, img)
	default:
		t.Fatalf("unsupported test format: %s", format)
	}

	if err != nil {
		t.Fatalf("encode image: %v", err)
	}

	return buf.Bytes()
}

func createTestFileHeader(
	t *testing.T,
	filename string,
	data []byte,
) *multipart.FileHeader {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("images", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}

	if _, err := part.Write(data); err != nil {
		t.Fatalf("write part: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/upload",
		&body,
	)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if err := req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}

	t.Cleanup(func() {
		if req.MultipartForm != nil {
			_ = req.MultipartForm.RemoveAll()
		}
	})

	headers := req.MultipartForm.File["images"]
	if len(headers) != 1 {
		t.Fatalf("expected 1 file header, got %d", len(headers))
	}

	return headers[0]
}

func TestValidateMainImages(t *testing.T) {
	jpegData := createTestImage(t, "jpeg")

	file1 := createTestFileHeader(t, "1.jpg", jpegData)
	file2 := createTestFileHeader(t, "2.jpg", jpegData)

	tests := []struct {
		name    string
		files   []*multipart.FileHeader
		wantErr error
	}{
		{
			name:  "two images",
			files: []*multipart.FileHeader{file1, file2},
		},
		{
			name:    "no images",
			files:   nil,
			wantErr: ErrInvalidImageCount,
		},
		{
			name:    "one image",
			files:   []*multipart.FileHeader{file1},
			wantErr: ErrInvalidImageCount,
		},
		{
			name: "three images",
			files: []*multipart.FileHeader{
				file1,
				file2,
				file1,
			},
			wantErr: ErrInvalidImageCount,
		},
		{
			name: "nil file",
			files: []*multipart.FileHeader{
				file1,
				nil,
			},
			wantErr: ErrInvalidImage,
		},
		{
			name: "empty file",
			files: []*multipart.FileHeader{
				file1,
				{Size: 0},
			},
			wantErr: ErrInvalidImage,
		},
		{
			name: "oversized file",
			files: []*multipart.FileHeader{
				file1,
				{Size: MaxImageSize + 1},
			},
			wantErr: ErrImageTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMainImages(tt.files)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf(
					"expected %v, got %v",
					tt.wantErr,
					err,
				)
			}
		})
	}
}

func TestValidateImage(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantFormat string
		wantExt    string
		wantErr    error
	}{
		{
			name:       "valid jpeg",
			data:       createTestImage(t, "jpeg"),
			wantFormat: "image/jpeg",
			wantExt:    ".jpg",
		},
		{
			name:       "valid png",
			data:       createTestImage(t, "png"),
			wantFormat: "image/png",
			wantExt:    ".png",
		},
		{
			name:    "empty data",
			data:    nil,
			wantErr: ErrInvalidImage,
		},
		{
			name:    "text data",
			data:    []byte("not an image"),
			wantErr: ErrInvalidImage,
		},
		{
			name: "truncated jpeg",
			data: []byte{
				0xFF, 0xD8, 0xFF, 0xE0,
				0x00, 0x10,
			},
			wantErr: ErrInvalidImage,
		},
		{
			name: "truncated png",
			data: []byte{
				0x89, 0x50, 0x4E, 0x47,
				0x0D, 0x0A, 0x1A, 0x0A,
			},
			wantErr: ErrInvalidImage,
		},
		{
			name:    "oversized data",
			data:    bytes.Repeat([]byte("a"), int(MaxImageSize)+1),
			wantErr: ErrImageTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateImage(
				multipartFileFromBytes{Reader: bytes.NewReader(tt.data)},
			)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf(
						"expected %v, got %v",
						tt.wantErr,
						err,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.ContentType != tt.wantFormat {
				t.Errorf(
					"expected content type %s, got %s",
					tt.wantFormat,
					result.ContentType,
				)
			}

			if result.Extension != tt.wantExt {
				t.Errorf(
					"expected extension %s, got %s",
					tt.wantExt,
					result.Extension,
				)
			}

			if result.Width != 10 || result.Height != 10 {
				t.Errorf(
					"expected 10x10, got %dx%d",
					result.Width,
					result.Height,
				)
			}

			if !bytes.Equal(result.Data, tt.data) {
				t.Error("validated image data does not match")
			}
		})
	}
}

// multipart.Fileをメモリ上のデータで再現する。
type multipartFileFromBytes struct {
	*bytes.Reader
}

func (multipartFileFromBytes) Close() error {
	return nil
}
