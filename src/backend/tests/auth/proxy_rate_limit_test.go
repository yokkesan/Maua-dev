package auth_test

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	infocontroller "backend/controllers/info"
	resetcontroller "backend/controllers/passwordreset"
	"backend/internal/adminauth"
	"backend/internal/clientip"
	beegoctx "github.com/beego/beego/v2/server/web/context"
)

func proxyTestIP(t *testing.T) string {
	t.Helper()
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		t.Fatal(err)
	}
	copy(bytes[:4], []byte{0x20, 0x01, 0x0d, 0xb8})
	return netip.AddrFrom16(bytes).String()
}
func TestProxyIPRateLimits(t *testing.T) {
	for _, endpoint := range []string{"info-login", "reset-request", "reset-confirm"} {
		for _, trusted := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s_trusted_%t", endpoint, trusted), func(t *testing.T) {
				peer, client, other := proxyTestIP(t), proxyTestIP(t), proxyTestIP(t)
				config := ""
				if trusted {
					config = peer
				}
				resolver, err := clientip.New(config)
				if err != nil {
					t.Fatal(err)
				}
				keys := map[string]bool{peer: true, client: true, other: true}
				t.Cleanup(func() {
					hashes := [][]byte{}
					for key := range keys {
						h := sha256.Sum256([]byte(key))
						hashes = append(hashes, h[:])
					}
					workflowExec(t, `DELETE FROM security_request_limits WHERE scope=ANY($1::text[]) AND key_hash=ANY($2::bytea[])`, []string{"info-login-ip", "info-login-email", "password-reset-ip", "password-reset-email", "password-reset-confirm"}, hashes)
				})
				request := func(xff, remote string) *httptest.ResponseRecorder {
					var nonce [12]byte
					if _, err := rand.Read(nonce[:]); err != nil {
						t.Fatal(err)
					}
					email := "missing-" + hex.EncodeToString(nonce[:]) + "@example.com"
					keys[email] = true
					body := fmt.Sprintf(`{"email":%q,"password":"invalid-test-password"}`, email)
					if endpoint == "reset-request" {
						body = fmt.Sprintf(`{"email":%q}`, email)
					}
					if endpoint == "reset-confirm" {
						body = `{"token":"invalid-token","password":"new-password"}`
					}
					req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(body))
					req.RemoteAddr = net.JoinHostPort(remote, "1234")
					req.Header.Set("X-Forwarded-For", xff)
					req.Header.Set("X-Real-IP", other)
					rec := httptest.NewRecorder()
					ctx := beegoctx.NewContext()
					ctx.Reset(rec, req)
					if endpoint == "info-login" {
						c := &infocontroller.Controller{Auth: &adminauth.Service{Repository: &adminauth.Repository{DB: testDB}}, ClientIP: resolver}
						c.Init(ctx, "Controller", "Login", nil)
						c.Login()
					} else {
						c := &resetcontroller.Controller{Service: resetService(), ClientIP: resolver}
						c.Init(ctx, "Controller", "Request", nil)
						if endpoint == "reset-request" {
							c.Request()
						} else {
							c.Reset()
						}
					}
					return rec
				}
				expected := 401
				if endpoint == "reset-request" {
					expected = 202
				}
				if endpoint == "reset-confirm" {
					expected = 400
				}
				for i := 0; i < 30; i++ {
					// A new spoofed left-hand hop cannot change the true client bucket.
					spoof := proxyTestIP(t)
					workflowStatus(t, request(spoof+", "+client, peer), expected)
				}
				workflowStatus(t, request(other+", "+client, peer), 429)
				if trusted {
					// Another real client behind the same trusted proxy has its own allowance.
					workflowStatus(t, request(other, peer), expected)
				} else {
					// Changing only spoofed headers does not evade a direct client's limit.
					workflowStatus(t, request(other, peer), 429)
					workflowStatus(t, request(client, other), expected)
				}
			})
		}
	}
}
