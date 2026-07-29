package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsjodin/via_go/internal/api"
	"github.com/dsjodin/via_go/internal/auth"
	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
)

func login(t *testing.T, r *gin.Engine, username, password string) *httptest.ResponseRecorder {
	t.Helper()

	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatalf("no %s cookie in the response", auth.CookieName)
	return nil
}

func withCookie(r *gin.Engine, method, path string, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestLoginIssuesAUsableSession(t *testing.T) {
	apiRouter, _ := newRouters(t)

	rec := login(t, apiRouter, "admin", "VMware1!")
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d, body %s", rec.Code, rec.Body.String())
	}

	cookie := sessionCookie(t, rec)
	if cookie.Value == "" {
		t.Fatal("empty session cookie")
	}

	if code := withCookie(apiRouter, http.MethodGet, "/v1/hosts", cookie).Code; code != http.StatusOK {
		t.Errorf("GET /v1/hosts with a session cookie = %d, want 200", code)
	}
}

// The token must not be readable from JavaScript, or an injected script can
// lift it straight out of document.cookie.
func TestSessionCookieIsHttpOnly(t *testing.T) {
	apiRouter, _ := newRouters(t)

	cookie := sessionCookie(t, login(t, apiRouter, "admin", "VMware1!"))

	if !cookie.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("session cookie SameSite = %v, want Strict", cookie.SameSite)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	apiRouter, _ := newRouters(t)

	for _, tc := range []struct{ user, pass string }{
		{"admin", "wrong"},
		{"nobody", "VMware1!"},
	} {
		rec := login(t, apiRouter, tc.user, tc.pass)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("login %s/%s = %d, want 401", tc.user, tc.pass, rec.Code)
		}
		// The failure must not reveal whether the account exists.
		if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("invalid username or password")) {
			t.Errorf("login failure leaks which half was wrong: %s", body)
		}
	}
}

// Basic auth has to keep working: the example scripts use it and the README
// promises everything doable in the UI is doable from automation.
func TestBasicAuthStillWorks(t *testing.T) {
	apiRouter, _ := newRouters(t)

	if code := get(apiRouter, "/v1/hosts", true).Code; code != http.StatusOK {
		t.Errorf("GET /v1/hosts with basic auth = %d, want 200", code)
	}
}

func TestLogoutInvalidatesTheSessionServerSide(t *testing.T) {
	apiRouter, _ := newRouters(t)

	cookie := sessionCookie(t, login(t, apiRouter, "admin", "VMware1!"))

	if code := withCookie(apiRouter, http.MethodPost, "/v1/logout", cookie).Code; code != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", code)
	}

	// Replaying the same cookie must fail, not merely be forgotten by the
	// browser.
	if code := withCookie(apiRouter, http.MethodGet, "/v1/hosts", cookie).Code; code != http.StatusUnauthorized {
		t.Errorf("the session cookie still worked after logout: %d", code)
	}
}

func TestSessionEndpointReportsTheUser(t *testing.T) {
	apiRouter, _ := newRouters(t)

	cookie := sessionCookie(t, login(t, apiRouter, "admin", "VMware1!"))
	rec := withCookie(apiRouter, http.MethodGet, "/v1/session", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("session = %d, body %s", rec.Code, rec.Body.String())
	}

	var got api.SessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Username != "admin" {
		t.Errorf("username = %q, want admin", got.Username)
	}
}

// A seeded account still on the documented default has to be visible to the
// UI so it can force a change.
func TestSessionReportsTheDefaultPassword(t *testing.T) {
	apiRouter, _ := newRouters(t)
	store.DB.Model(&model.User{}).Where("username = ?", "admin").Update("must_change_password", true)

	cookie := sessionCookie(t, login(t, apiRouter, "admin", "VMware1!"))
	rec := withCookie(apiRouter, http.MethodGet, "/v1/session", cookie)

	var got api.SessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.MustChangePassword {
		t.Error("session did not report that the default password is still in use")
	}
}

// Repeated guessing must be throttled — this appliance ships a documented
// default account and stores ESXi root passwords.
func TestRepeatedFailuresAreThrottled(t *testing.T) {
	apiRouter, _ := newRouters(t)

	var blocked bool
	for range 15 {
		if login(t, apiRouter, "admin", "wrong").Code == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}

	if !blocked {
		t.Fatal("never throttled after 15 failed logins")
	}

	// The block must hold even once the right password is offered, otherwise
	// it does not slow guessing down at all.
	if code := login(t, apiRouter, "admin", "VMware1!").Code; code != http.StatusTooManyRequests {
		t.Errorf("throttle lifted as soon as the correct password arrived: %d", code)
	}
}

func TestBasicAuthIsThrottledToo(t *testing.T) {
	apiRouter, _ := newRouters(t)

	var blocked bool
	for range 15 {
		req := httptest.NewRequest(http.MethodGet, "/v1/hosts", nil)
		req.SetBasicAuth("admin", "wrong")
		rec := httptest.NewRecorder()
		apiRouter.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}

	if !blocked {
		t.Error("basic auth is not throttled; it is a guessing endpoint on every route")
	}
}

// The plain HTTP boot engine must not expose the login endpoint — credentials
// would cross the wire unencrypted.
func TestBootRouterHasNoLogin(t *testing.T) {
	_, bootRouter := newRouters(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/login", bytes.NewBufferString(`{"username":"admin","password":"VMware1!"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	bootRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("boot router serves /v1/login (%d); credentials would cross plain HTTP", rec.Code)
	}
}

// Re-running post-configuration by hand is an operator action, so the by-id
// form must stay behind authentication even though the host's own report does
// not.
func TestPostconfigByIDRequiresAuth(t *testing.T) {
	apiRouter, _ := newRouters(t)

	if code := get(apiRouter, "/v1/postconfig/1", false).Code; code != http.StatusUnauthorized {
		t.Errorf("GET /v1/postconfig/1 without credentials = %d, want 401", code)
	}
}
