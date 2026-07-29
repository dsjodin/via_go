package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/secrets"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
)

func groupRouter() *gin.Engine {
	r := gin.New()
	r.GET("/v1/groups", ListGroups)
	r.GET("/v1/groups/:id", GetGroup)
	r.POST("/v1/groups", CreateGroup(testKey))
	r.PATCH("/v1/groups/:id", UpdateGroup(testKey))
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// The stored password is ciphertext, but it is still a secret: it is the only
// thing standing between a read-only API consumer and every ESXi root password
// once they also have the key file. No response may carry it.
func TestGroupResponsesNeverCarryThePassword(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())
	r := groupRouter()

	create := `{"name":"probe","dns":"10.0.0.53","password":"VMware1!","netmask":"255.255.255.0","gateway":"192.168.1.1"}`

	responses := map[string]*httptest.ResponseRecorder{
		"create": do(t, r, http.MethodPost, "/v1/groups", create),
		"list":   do(t, r, http.MethodGet, "/v1/groups", ""),
		"get":    do(t, r, http.MethodGet, "/v1/groups/1", ""),
		"update": do(t, r, http.MethodPatch, "/v1/groups/1", `{"name":"renamed","password":"VMware2!"}`),
	}

	for name, rec := range responses {
		if rec.Code >= 400 {
			t.Fatalf("%s: status %d, body %s", name, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"password"`) {
			t.Errorf("%s response contains a password field:\n%s", name, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "VMware1!") || strings.Contains(rec.Body.String(), "VMware2!") {
			t.Errorf("%s response contains a plaintext password:\n%s", name, rec.Body.String())
		}
	}
}

// Hiding the password from responses must not stop it being set.
func TestCreateGroupStoresAnEncryptedPassword(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())
	r := groupRouter()

	rec := do(t, r, http.MethodPost, "/v1/groups",
		`{"name":"probe","password":"VMware1!","netmask":"255.255.255.0","gateway":"192.168.1.1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var stored model.Group
	if res := store.DB.First(&stored, created.ID); res.Error != nil {
		t.Fatalf("reload group: %v", res.Error)
	}

	if stored.Password == "" {
		t.Fatal("no password was stored")
	}
	if stored.Password == "VMware1!" {
		t.Fatal("the password was stored in plaintext")
	}

	got, err := secrets.Decrypt(stored.Password, testKey)
	if err != nil {
		t.Fatalf("stored password does not decrypt: %v", err)
	}
	if got != "VMware1!" {
		t.Errorf("decrypted password = %q, want %q", got, "VMware1!")
	}
}

// Updating a group used to rely on mergo copying the password through the
// embedded form struct. With the password no longer embedded, a change must
// still take effect — and must be encrypted, not double-encrypted.
func TestUpdateGroupReencryptsANewPassword(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())
	r := groupRouter()

	rec := do(t, r, http.MethodPatch, "/v1/groups/1", `{"password":"NewPass1!"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}

	var stored model.Group
	if res := store.DB.First(&stored, 1); res.Error != nil {
		t.Fatalf("reload group: %v", res.Error)
	}

	got, err := secrets.Decrypt(stored.Password, testKey)
	if err != nil {
		t.Fatalf("stored password does not decrypt: %v", err)
	}
	if got != "NewPass1!" {
		t.Errorf("decrypted password = %q, want %q — the update did not take effect cleanly", got, "NewPass1!")
	}
}

// An update that does not mention the password must leave it untouched, or
// every unrelated edit would wipe the host's root password.
func TestUpdateGroupWithoutAPasswordLeavesItAlone(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())
	r := groupRouter()

	var before model.Group
	store.DB.First(&before, 1)

	rec := do(t, r, http.MethodPatch, "/v1/groups/1", `{"name":"renamed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}

	var after model.Group
	store.DB.First(&after, 1)

	if after.Password != before.Password {
		t.Errorf("password changed on an update that did not supply one")
	}
	if after.Name != "renamed" {
		t.Errorf("name = %q, want %q", after.Name, "renamed")
	}
}

// The kickstart handler reads the password as a Go field, so hiding it from
// JSON must not affect it. This is the path that would silently install hosts
// with the wrong root password.
func TestKickstartStillSeesThePasswordAfterHidingItFromJSON(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())

	body := render(t, "192.168.1.50").Body.String()
	if !strings.Contains(body, "rootpw VMware1!") {
		t.Errorf("kickstart lost the root password:\n%s", body)
	}
}

func TestCreateGroupRejectsAWeakPassword(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())
	r := groupRouter()

	rec := do(t, r, http.MethodPost, "/v1/groups", `{"name":"weak","password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a password that fails complexity", rec.Code)
	}
}
