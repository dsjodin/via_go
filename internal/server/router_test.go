package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/dsjodin/via_go/internal/api"
	"github.com/dsjodin/via_go/internal/config"
	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
)

var routerSeq uint64

func newRouters(t *testing.T) (apiRouter, bootRouter *gin.Engine) {
	t.Helper()
	return newRoutersWithUI(t, nil)
}

// newRoutersWithUI builds the routers over a given frontend; nil uses the
// embedded one.
func newRoutersWithUI(t *testing.T, ui fs.FS) (apiRouter, bootRouter *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:router%d?mode=memory&cache=shared", atomic.AddUint64(&routerSeq, 1))
	if err := store.Open(dsn, false); err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := store.DB.AutoMigrate(&model.Host{}, &model.Group{}, &model.Image{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user := model.User{
		UserForm: model.UserForm{Username: "admin"},
		Password: api.HashAndSalt([]byte("VMware1!")),
	}
	if res := store.DB.Create(&user); res.Error != nil {
		t.Fatalf("create user: %v", res.Error)
	}

	a, b, err := NewRouters(Options{
		Config:    &config.Config{Port: 8443},
		SecretKey: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		UI:        ui,
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
		// A freshly installed host reports completion here at the end of its
		// kickstart, with no credentials to offer.
		{"install completion", apiRouter, "/v1/postconfig"},
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

func TestUIAndSwaggerAreServed(t *testing.T) {
	apiRouter, _ := newRouters(t)

	if code := get(apiRouter, "/web/", true).Code; code != http.StatusOK {
		t.Errorf("GET /web/ = %d, want 200", code)
	}
	if code := get(apiRouter, "/swagger/index.html", true).Code; code != http.StatusOK {
		t.Errorf("GET /swagger/index.html = %d, want 200", code)
	}
}

// The login screen is part of the UI bundle, so the bundle cannot sit behind
// the session it exists to create. Putting it there makes the whole product
// unreachable: a browser gets a bare 401 and no way to log in.
func TestUIIsReachableWithoutCredentials(t *testing.T) {
	apiRouter, _ := newRouters(t)

	for _, tc := range []struct {
		name string
		path string
		want int
	}{
		{"entry point", "/", http.StatusFound},
		{"app shell", "/web/", http.StatusOK},
		{"unknown path", "/hosts", http.StatusFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code := get(apiRouter, tc.path, false).Code; code != tc.want {
				t.Errorf("GET %s without credentials = %d, want %d", tc.path, code, tc.want)
			}
		})
	}

	// The login route itself only exists in a real UI build; webui/dist holds a
	// placeholder here. 404 is therefore fine, 401 is the regression.
	t.Run("login screen", func(t *testing.T) {
		if code := get(apiRouter, "/web/login/", false).Code; code == http.StatusUnauthorized {
			t.Error("GET /web/login/ = 401; the login screen must not require a session")
		}
	})
}

// exportFS mirrors the shape of a real `next build` export: trailingSlash puts
// one index.html per route inside a directory of that name, and the shared
// chunks live under _next. The committed webui/dist is a single placeholder
// page, so it cannot catch a routing mistake that only shows up on the
// non-root pages — which is exactly the mistake that has been made here.
func exportFS() fs.FS {
	return fstest.MapFS{
		"index.html":            {Data: []byte("<html>root</html>")},
		"images/index.html":     {Data: []byte("<html>images</html>")},
		"login/index.html":      {Data: []byte("<html>login</html>")},
		"_next/static/chunk.js": {Data: []byte("console.log(1)")},
		"404.html":              {Data: []byte("<html>404</html>")},
	}
}

// A reload or a typed URL on any page but the root must work. Client-side
// navigation hides a failure here, because next/link never asks the server for
// the document — so this only surfaces when someone refreshes, and then the
// whole page is gone.
func TestExportedRoutesAreServedOnAFullPageLoad(t *testing.T) {
	apiRouter, _ := newRoutersWithUI(t, exportFS())

	t.Run("page", func(t *testing.T) {
		rec := get(apiRouter, "/web/images/", false)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /web/images/ = %d, want 200", rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, "images") {
			t.Errorf("served %q, want the images document", body)
		}
	})

	// Without the trailing slash the file server redirects. The Location it
	// sends is relative, so the browser resolves it against /web/images and
	// keeps the prefix; an absolute one would drop out of /web entirely.
	t.Run("without a trailing slash", func(t *testing.T) {
		rec := get(apiRouter, "/web/images", false)
		if rec.Code != http.StatusMovedPermanently {
			t.Fatalf("GET /web/images = %d, want 301", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "images/" {
			t.Errorf("redirected to %q, want the relative %q", loc, "images/")
		}
	})

	t.Run("shared chunks", func(t *testing.T) {
		if code := get(apiRouter, "/web/_next/static/chunk.js", false).Code; code != http.StatusOK {
			t.Errorf("GET a _next chunk = %d, want 200", code)
		}
	})
}

// The entry point people actually type is the bare address, and anything else
// outside /web is not an app route at all. Both land on the app.
func TestPathsOutsideTheUIRedirectToIt(t *testing.T) {
	apiRouter, _ := newRouters(t)

	for _, path := range []string{"/", "/hosts/1/edit"} {
		rec := get(apiRouter, path, false)
		if got := rec.Header().Get("Location"); got != "/web/" {
			t.Errorf("GET %s redirected to %q, want %q", path, got, "/web/")
		}
	}
}
