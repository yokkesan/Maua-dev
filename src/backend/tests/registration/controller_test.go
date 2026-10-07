package registration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	beego "github.com/beego/beego/v2/server/web"
)

type verificationCreateResponse struct {
	Message   string `json:"message"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type registrationCompleteResponse struct {
	Message   string `json:"message"`
	CompanyID int64  `json:"company_id"`
	UserID    int64  `json:"user_id"`
	BrandID   int64  `json:"brand_id"`
	ShopID    int64  `json:"shop_id"`
}

func TestRegistrationVerificationEndpoint(t *testing.T) {
	body := []byte(`{
		"email": "controller-test@example.com",
		"company_name": "テスト株式会社",
		"company_phonetic": "テストカブシキガイシャ",
		"company_post_code": "100-0001",
		"company_address": "東京都千代田区",
		"company_tel": "03-1234-5678",
		"company_type": 1,
		"representative_name": "山田太郎",
		"representative_phonetic": "ヤマダタロウ",
		"registration_plan": 0
	}`)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/verification",
		bytes.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if !bytes.Contains(
		recorder.Body.Bytes(),
		[]byte(`"verification token created"`),
	) {
		t.Fatalf(
			"unexpected response body: %s",
			recorder.Body.String(),
		)
	}
}

func TestRegistrationVerificationRejectsInvalidEmail(t *testing.T) {
	body := []byte(`{
		"email": "invalid-email",
		"company_name": "テスト株式会社",
		"company_type": 1,
		"registration_plan": 0
	}`)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/verification",
		bytes.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestRegistrationVerificationGetValidToken(t *testing.T) {
	body := []byte(`{
		"email": "controller-valid-token@example.com",
		"company_name": "テスト株式会社",
		"company_phonetic": "テストカブシキガイシャ",
		"company_post_code": "100-0001",
		"company_address": "東京都千代田区",
		"company_tel": "03-1234-5678",
		"company_type": 1,
		"representative_name": "山田太郎",
		"representative_phonetic": "ヤマダタロウ",
		"registration_plan": 0
	}`)

	postReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/verification",
		bytes.NewReader(body),
	)

	postReq.Header.Set("Content-Type", "application/json")

	postRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		postRecorder,
		postReq,
	)

	if postRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected POST status %d, got %d: %s",
			http.StatusCreated,
			postRecorder.Code,
			postRecorder.Body.String(),
		)
	}

	var createResponse verificationCreateResponse

	if err := json.Unmarshal(
		postRecorder.Body.Bytes(),
		&createResponse,
	); err != nil {
		t.Fatalf(
			"failed to decode POST response: %v",
			err,
		)
	}

	if createResponse.Token == "" {
		t.Fatal("verification token should not be empty")
	}

	getReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/registration/verification?token="+url.QueryEscape(createResponse.Token),
		nil,
	)

	getRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		getRecorder,
		getReq,
	)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf(
			"expected GET status %d, got %d: %s",
			http.StatusOK,
			getRecorder.Code,
			getRecorder.Body.String(),
		)
	}

	if !bytes.Contains(
		getRecorder.Body.Bytes(),
		[]byte(`"verification token is valid"`),
	) {
		t.Fatalf(
			"unexpected response body: %s",
			getRecorder.Body.String(),
		)
	}

	if !bytes.Contains(
		getRecorder.Body.Bytes(),
		[]byte(`"controller-valid-token@example.com"`),
	) {
		t.Fatalf(
			"response does not contain expected email: %s",
			getRecorder.Body.String(),
		)
	}
}

func TestRegistrationVerificationGetRejectsInvalidToken(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/registration/verification?token=invalid-token",
		nil,
	)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if !bytes.Contains(
		recorder.Body.Bytes(),
		[]byte(`"invalid verification token"`),
	) {
		t.Fatalf(
			"unexpected response body: %s",
			recorder.Body.String(),
		)
	}
}

func TestRegistrationVerificationGetRejectsRevokedToken(t *testing.T) {
	firstBody := []byte(`{
		"email": "controller-revoked-token@example.com",
		"company_name": "テスト株式会社",
		"company_type": 1,
		"registration_plan": 0
	}`)

	firstReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/verification",
		bytes.NewReader(firstBody),
	)

	firstReq.Header.Set("Content-Type", "application/json")

	firstRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		firstRecorder,
		firstReq,
	)

	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected first POST status %d, got %d: %s",
			http.StatusCreated,
			firstRecorder.Code,
			firstRecorder.Body.String(),
		)
	}

	var firstResponse verificationCreateResponse

	if err := json.Unmarshal(
		firstRecorder.Body.Bytes(),
		&firstResponse,
	); err != nil {
		t.Fatalf(
			"failed to decode first POST response: %v",
			err,
		)
	}

	if firstResponse.Token == "" {
		t.Fatal("first verification token should not be empty")
	}

	secondBody := []byte(`{
		"email": "controller-revoked-token@example.com",
		"company_name": "テスト株式会社",
		"company_type": 1,
		"registration_plan": 0
	}`)

	secondReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/verification",
		bytes.NewReader(secondBody),
	)

	secondReq.Header.Set("Content-Type", "application/json")

	secondRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		secondRecorder,
		secondReq,
	)

	if secondRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected second POST status %d, got %d: %s",
			http.StatusCreated,
			secondRecorder.Code,
			secondRecorder.Body.String(),
		)
	}

	getReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/registration/verification?token="+url.QueryEscape(firstResponse.Token),
		nil,
	)

	getRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		getRecorder,
		getReq,
	)

	if getRecorder.Code != http.StatusGone {
		t.Fatalf(
			"expected GET status %d, got %d: %s",
			http.StatusGone,
			getRecorder.Code,
			getRecorder.Body.String(),
		)
	}

	if !bytes.Contains(
		getRecorder.Body.Bytes(),
		[]byte(`"verification token has been revoked"`),
	) {
		t.Fatalf(
			"unexpected response body: %s",
			getRecorder.Body.String(),
		)
	}
}

func TestRegistrationCompleteBrand(t *testing.T) {
	verificationBody := []byte(`{
		"email": "controller-complete-brand@example.com",
		"company_name": "本登録テスト株式会社",
		"company_phonetic": "ホントウロクテストカブシキガイシャ",
		"company_post_code": "100-0001",
		"company_address": "東京都千代田区",
		"company_tel": "03-1234-5678",
		"company_type": 1,
		"representative_name": "山田太郎",
		"representative_phonetic": "ヤマダタロウ",
		"registration_plan": 0
	}`)

	verificationReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/verification",
		bytes.NewReader(verificationBody),
	)

	verificationReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	verificationRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		verificationRecorder,
		verificationReq,
	)

	if verificationRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected verification status %d, got %d: %s",
			http.StatusCreated,
			verificationRecorder.Code,
			verificationRecorder.Body.String(),
		)
	}

	var verificationResponse verificationCreateResponse

	if err := json.Unmarshal(
		verificationRecorder.Body.Bytes(),
		&verificationResponse,
	); err != nil {
		t.Fatalf(
			"failed to decode verification response: %v",
			err,
		)
	}

	if verificationResponse.Token == "" {
		t.Fatal("verification token should not be empty")
	}

	completeBody, err := json.Marshal(
		map[string]string{
			"token":    verificationResponse.Token,
			"password": "password123",
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode complete request: %v",
			err,
		)
	}

	completeReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/complete",
		bytes.NewReader(completeBody),
	)

	completeReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	completeRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		completeRecorder,
		completeReq,
	)

	if completeRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected complete status %d, got %d: %s",
			http.StatusCreated,
			completeRecorder.Code,
			completeRecorder.Body.String(),
		)
	}

	var completeResponse registrationCompleteResponse

	if err := json.Unmarshal(
		completeRecorder.Body.Bytes(),
		&completeResponse,
	); err != nil {
		t.Fatalf(
			"failed to decode complete response: %v",
			err,
		)
	}

	if completeResponse.Message != "registration completed" {
		t.Fatalf(
			"unexpected message: %s",
			completeResponse.Message,
		)
	}

	if completeResponse.CompanyID == 0 {
		t.Fatal("company_id should not be zero")
	}

	if completeResponse.UserID == 0 {
		t.Fatal("user_id should not be zero")
	}

	if completeResponse.BrandID == 0 {
		t.Fatal("brand_id should not be zero")
	}

	if completeResponse.ShopID != 0 {
		t.Fatal("shop_id should not be returned for brand registration")
	}
}

func TestRegistrationCompleteRejectsInvalidToken(t *testing.T) {
	body := []byte(`{
		"token": "invalid-registration-token",
		"password": "password123"
	}`)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/complete",
		bytes.NewReader(body),
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		recorder,
		req,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d: %s",
			http.StatusBadRequest,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if !bytes.Contains(
		recorder.Body.Bytes(),
		[]byte(`"invalid verification token"`),
	) {
		t.Fatalf(
			"unexpected response body: %s",
			recorder.Body.String(),
		)
	}
}

func TestRegistrationCompleteRejectsConsumedToken(t *testing.T) {
	verificationBody := []byte(`{
		"email": "controller-consumed-token@example.com",
		"company_name": "使用済みテスト株式会社",
		"company_type": 1,
		"registration_plan": 0
	}`)

	verificationReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/verification",
		bytes.NewReader(verificationBody),
	)

	verificationReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	verificationRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		verificationRecorder,
		verificationReq,
	)

	if verificationRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected verification status %d, got %d: %s",
			http.StatusCreated,
			verificationRecorder.Code,
			verificationRecorder.Body.String(),
		)
	}

	var verificationResponse verificationCreateResponse

	if err := json.Unmarshal(
		verificationRecorder.Body.Bytes(),
		&verificationResponse,
	); err != nil {
		t.Fatalf(
			"failed to decode verification response: %v",
			err,
		)
	}

	completeBody, err := json.Marshal(
		map[string]string{
			"token":    verificationResponse.Token,
			"password": "password123",
		},
	)
	if err != nil {
		t.Fatalf(
			"failed to encode complete request: %v",
			err,
		)
	}

	firstReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/complete",
		bytes.NewReader(completeBody),
	)

	firstReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	firstRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		firstRecorder,
		firstReq,
	)

	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected first complete status %d, got %d: %s",
			http.StatusCreated,
			firstRecorder.Code,
			firstRecorder.Body.String(),
		)
	}

	secondReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/registration/complete",
		bytes.NewReader(completeBody),
	)

	secondReq.Header.Set(
		"Content-Type",
		"application/json",
	)

	secondRecorder := httptest.NewRecorder()

	beego.BeeApp.Handlers.ServeHTTP(
		secondRecorder,
		secondReq,
	)

	if secondRecorder.Code != http.StatusGone {
		t.Fatalf(
			"expected second complete status %d, got %d: %s",
			http.StatusGone,
			secondRecorder.Code,
			secondRecorder.Body.String(),
		)
	}

	if !bytes.Contains(
		secondRecorder.Body.Bytes(),
		[]byte(`"verification token has already been used"`),
	) {
		t.Fatalf(
			"unexpected response body: %s",
			secondRecorder.Body.String(),
		)
	}
}