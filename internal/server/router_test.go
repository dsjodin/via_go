package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dsjodin/via_go/internal/api"
	"github.com/dsjodin/via_go/internal/config"
	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
)

var routerSeq uint64

func newRouters(t *testing.T) (apiRouter, bootRouter *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:router%d?mode=memory&cache=shared", atomic.AddUint64(&routerSeq, 1))
	if err := store.Open(dsn, false); err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := store.DB.AutoMigrate(&model.Host{}, &model.Group{}, &model.Image{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user := model.User{UserForm: model.UserForm{
		Username: "admin",
		Password: api.HashAndSalt([]byte("VMware1!")),
	}}
	if res := store.DB.Create(&user); res.Error != nil {
		t.Fatalf("create user: %v", res.Error)
	}

	a, b, err := NewRouters(Options{
		Config:    &config.Config{Port: 8443},
		SecretKey: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		Version:   Version{Version: "test", Commit: "none", Date: "unknown"},
	})
	if err != nil {
		t.Fatalf("NewRouters: %v", err)
	}
	return a, b
}

func get(r *gin.Engine, path string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth {
		req.SetBasicAuth("admin", "VMware1!")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// Every API route must be behind authentication. A regression here exposes
// host records, group passwords and the whole configuration surface.
func TestAPIRoutesRequireAuthentication(t *testing.T) {
	apiRouter, _ := newRouters(t)

	for _, path := range []string{
		"/v1/hosts",
		"/v1/groups",
		"/v1/images",
		"/v1/users",
		"/v1/device_classes",
		"/v1/version",
	} {
		if code := get(apiRouter, path, false).Code; code != http.StatusUnauthorized {
			t.Errorf("GET %s without credentials = %d, want 401", path, code)
		}
	}
}

func TestAPIRoutesAcceptValidCredentials(t *testing.T) {
	apiRouter, _ := newRouters(t)

	if code := get(apiRouter, "/v1/version", true).Code; code != http.StatusOK {
		t.Errorf("GET /v1/version with credentials = %d, want 200", code)
	}
}

func TestAPIRoutesRejectWrongPassword(t *testing.T) {
	apiRouter, _ := newRouters(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	req.SetBasicAuth("admin", "wrong")
	rec := httptest.NewRecorder()
	apiRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("GET /v1/version with a wrong password = %d, want 401", rec.Code)
	}
}

// Two routes deliberately sit in front of the auth middleware, because the
// clients using them cannot authenticate: an installer fetching its kickstart
// and a machine doing UEFI HTTP boot. They must stay reachable, and they must
// not be reachable by accident either — both are authorised by source address
// against a host record, so an unknown caller gets nothing useful.
func TestUnauthenticatedRoutesStayReachable(t *testing.T) {
	apiRouter, bootRouter := newRouters(t)

	for _, tc := range []struct {
		name   string
		router *gin.Engine
		path   string
	}{
		{"kickstart", apiRouter, "/ks.cfg"},
		{"boot files over https", apiRouter, "/esx/mboot.efi"},
		{"boot files over http", bootRouter, "/esx/mboot.efi"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code := get(tc.router, tc.path, false).Code; code == http.StatusUnauthorized {
				t.Errorf("GET %s = 401; this route must not be behind authentication", tc.path)
			}
		})
	}
}

// The boot router carries no API or UI surface at all — it listens on plain
// HTTP, so anything else exposed there would be unauthenticated over the wire.
func TestBootRouterExposesNothingElse(t *testing.T) {
	_, bootRouter := newRouters(t)

	for _, path := range []string{
		"/v1/hosts",
		"/v1/users",
		"/v1/version",
		"/ks.cfg",
		"/web/",
		"/swagger/index.html",
	} {
		if code := get(bootRouter, path, false).Code; code != http.StatusNotFound {
			t.Errorf("boot router serves %s (%d); it should only serve /esx/", path, code)
		}
	}
}

// Unknown paths return the single page app so it can route them client side.
func TestUnknownPathServesTheUI(t *testing.T) {
	apiRouter, _ := newRouters(t)

	rec := get(apiRouter, "/hosts/1/edit", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET an app route = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("no content type on the UI response")
	}
}

func TestUIAndSwaggerAreServed(t *testing.T) {
	apiRouter, _ := newRouters(t)

	if code := get(apiRouter, "/web/", true).Code; code != http.StatusOK {
		t.Errorf("GET /web/ = %d, want 200", code)
	}
	if code := get(apiRouter, "/swagger/index.html", true).Code; code != http.StatusOK {
		t.Errorf("GET /swagger/index.html = %d, want 200", code)
	}
}
