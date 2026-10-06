package registration_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "backend/routers"

	beego "github.com/beego/beego/v2/server/web"
)

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