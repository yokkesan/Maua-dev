package brandcontroller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"backend/internal/auth"
	"backend/internal/brand"
	"backend/internal/storage"

	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
)

type BrandCreateController struct {
	beego.Controller
	Service      *brand.Service
	ImageService *storage.MainImageService
}

func (c *BrandCreateController) Post() {
	user, ok := auth.UserFromContext(c.Ctx.Request.Context())
	if !ok || user == nil {
		c.respondError(http.StatusUnauthorized, "unauthorized")
		return
	}

	if c.Service == nil {
		c.respondError(
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	contentType, _, err := mime.ParseMediaType(
		c.Ctx.Request.Header.Get("Content-Type"),
	)

	// 従来のJSON登録は、既存処理を維持する。
	// Content-Type未指定時も従来と同じJSON解析を行う。
	if err != nil || contentType != "multipart/form-data" {
		c.createFromJSON(user.CompanyID)
		return
	}

	c.createFromMultipart(user.CompanyID)
}

// createFromJSON は従来のJSON形式による登録処理。
func (c *BrandCreateController) createFromJSON(
	companyID int64,
) {
	body := bytes.TrimSpace(c.Ctx.Input.RequestBody)

	input, err := decodeCreateBrandInput(body)
	if err != nil {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	result, err := c.Service.Create(
		c.Ctx.Request.Context(),
		companyID,
		input,
	)

	if err != nil {
		c.respondCreateError(err)
		return
	}

	c.respondCreated(result)
}

// createFromMultipart は画像2枚を含む登録処理。
func (c *BrandCreateController) createFromMultipart(
	companyID int64,
) {
	if c.ImageService == nil {
		c.respondError(
			http.StatusInternalServerError,
			"internal server error",
		)
		return
	}

	request := c.Ctx.Request

	// メモリ内に保持するmultipartデータの目安。
	// リクエスト全体の厳密な容量制限はNginx側でも設定する。
	const maxMemory = 12 << 20

	if err := request.ParseMultipartForm(maxMemory); err != nil {
		c.respondError(
			http.StatusBadRequest,
			"invalid multipart form",
		)
		return
	}

	if request.MultipartForm == nil {
		c.respondError(
			http.StatusBadRequest,
			"invalid multipart form",
		)
		return
	}

	form := request.MultipartForm

	// Goが一時ファイルを作成した場合は後片付けする。
	defer form.RemoveAll()

	// 登録情報はdataフィールドにJSONで指定する。
	dataFields := form.Value["data"]

	if len(dataFields) != 1 {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	input, err := decodeCreateBrandInput(
		[]byte(dataFields[0]),
	)
	if err != nil {
		c.respondError(
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	// main_imagesフィールドに画像2枚を指定する。
	files := form.File["main_images"]

	if err := validateMultipartBrandImages(files); err != nil {
		c.respondError(
			http.StatusBadRequest,
			"invalid main images",
		)
		return
	}

	result, err := c.Service.CreateWithImages(
		request.Context(),
		companyID,
		input,
		files,
		c.ImageService,
	)

	if err != nil {
		c.respondCreateError(err)
		return
	}

	c.respondCreated(result)
}

// decodeCreateBrandInput は既存JSONとmultipart内JSONに共通の解析処理。
// JSONオブジェクト以外、空本文、不正JSON、余分なJSON値を拒否する。
func decodeCreateBrandInput(
	body []byte,
) (brand.CreateBrandInput, error) {
	body = bytes.TrimSpace(body)

	if len(body) == 0 || body[0] != '{' {
		return nil, errors.New("invalid request body")
	}

	var input brand.CreateBrandInput

	decoder := json.NewDecoder(bytes.NewReader(body))

	if err := decoder.Decode(&input); err != nil {
		return nil, err
	}

	var extra any

	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("unexpected JSON value")
	}

	return input, nil
}

// validateMultipartBrandImages は画像2枚の形式・容量・破損を検証する。
// 不正画像をDB登録前に400として扱うための検証。
func validateMultipartBrandImages(
	files []*multipart.FileHeader,
) error {
	if err := storage.ValidateMainImages(files); err != nil {
		return err
	}

	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			return err
		}

		_, validateErr := storage.ValidateImage(file)
		closeErr := file.Close()

		if validateErr != nil {
			return validateErr
		}

		if closeErr != nil {
			return closeErr
		}
	}

	return nil
}

// respondCreateError は登録処理のエラーをHTTPレスポンスに変換する。
func (c *BrandCreateController) respondCreateError(err error) {
	if errors.Is(err, brand.ErrInvalidBrandInput) {
		c.respondError(
			http.StatusBadRequest,
			"invalid brand input",
		)
		return
	}

	// Cleanup失敗を含むエラー連鎖をサーバーログへ残す。
	logs.Error("create brand failed: %v", err)

	c.respondError(
		http.StatusInternalServerError,
		"internal server error",
	)
}

// respondCreated は登録成功時の共通レスポンス。
func (c *BrandCreateController) respondCreated(
	result map[string]any,
) {
	c.Ctx.Output.SetStatus(http.StatusCreated)
	c.Data["json"] = map[string]any{
		"brand": result,
	}
	_ = c.ServeJSON()
}

func (c *BrandCreateController) respondError(
	status int,
	message string,
) {
	c.Ctx.Output.SetStatus(status)
	c.Data["json"] = map[string]string{
		"error": message,
	}
	_ = c.ServeJSON()
}
