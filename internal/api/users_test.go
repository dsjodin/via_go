package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
)

func userRouter() *gin.Engine {
	r := gin.New()
	r.GET("/v1/users", ListUsers)
	r.GET("/v1/users/:id", GetUser)
	r.POST("/v1/users/search", SearchUser)
	r.POST("/v1/users", CreateUser)
	r.PATCH("/v1/users/:id", UpdateUser)
	r.DELETE("/v1/users/:id", DeleteUser)
	return r
}

func seedUser(t *testing.T) {
	t.Helper()
	seed(t, defaultGroup(), defaultHost())

	u := model.User{
		UserForm: model.UserForm{Username: "admin", Email: "admin@example.com"},
		Password: HashAndSalt([]byte("VMware1!")),
	}
	if res := store.DB.Create(&u); res.Error != nil {
		t.Fatalf("seed user: %v", res.Error)
	}
}

// The stored value is a bcrypt hash, which is still worth cracking offline.
// Every user endpoint used to return it.
func TestUserResponsesNeverCarryThePasswordHash(t *testing.T) {
	seedUser(t)
	r := userRouter()

	responses := map[string]string{
		"create": do(t, r, http.MethodPost, "/v1/users", `{"username":"bob","password":"Secret1!"}`).Body.String(),
		"list":   do(t, r, http.MethodGet, "/v1/users", "").Body.String(),
		"get":    do(t, r, http.MethodGet, "/v1/users/1", "").Body.String(),
		"update": do(t, r, http.MethodPatch, "/v1/users/1", `{"email":"new@example.com"}`).Body.String(),
	}

	for name, body := range responses {
		if strings.Contains(body, `"password"`) {
			t.Errorf("%s response contains a password field:\n%s", name, body)
		}
		if strings.Contains(body, "$2a$") || strings.Contains(body, "$2b$") {
			t.Errorf("%s response contains a bcrypt hash:\n%s", name, body)
		}
	}
}

// Updating any other field used to re-hash the stored hash, silently
// destroying the password. On the seeded admin account that locks you out of
// the appliance entirely.
func TestUpdateUserWithoutAPasswordLeavesItUsable(t *testing.T) {
	seedUser(t)
	r := userRouter()

	rec := do(t, r, http.MethodPatch, "/v1/users/1", `{"email":"new@example.com"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var after model.User
	store.DB.First(&after, 1)

	if !ComparePasswords(after.Password, []byte("VMware1!"), "admin") {
		t.Error("the password no longer validates after an update that did not supply one")
	}
	if after.Email != "new@example.com" {
		t.Errorf("email = %q, want new@example.com", after.Email)
	}
}

func TestUpdateUserWithAPasswordChangesIt(t *testing.T) {
	seedUser(t)
	r := userRouter()

	rec := do(t, r, http.MethodPatch, "/v1/users/1", `{"password":"Changed1!"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var after model.User
	store.DB.First(&after, 1)

	if !ComparePasswords(after.Password, []byte("Changed1!"), "admin") {
		t.Error("the new password does not validate")
	}
	if ComparePasswords(after.Password, []byte("VMware1!"), "admin") {
		t.Error("the old password still validates")
	}
}

func TestCreateUserStoresAHash(t *testing.T) {
	seedUser(t)
	r := userRouter()

	rec := do(t, r, http.MethodPost, "/v1/users", `{"username":"bob","password":"Secret1!"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var stored model.User
	store.DB.First(&stored, created.ID)

	if stored.Password == "Secret1!" {
		t.Fatal("the password was stored in plaintext")
	}
	if !ComparePasswords(stored.Password, []byte("Secret1!"), "bob") {
		t.Error("the stored hash does not validate against the supplied password")
	}
}

func TestUserSearchRejectsSQLInKeys(t *testing.T) {
	seedUser(t)
	r := userRouter()

	if code := do(t, r, http.MethodPost, "/v1/users/search", `{"username = ? OR 1=1":"nope"}`).Code; code == http.StatusOK {
		t.Error("user search executed SQL supplied in a key")
	}
	if code := do(t, r, http.MethodPost, "/v1/users/search", `{"username":"admin"}`).Code; code != http.StatusOK {
		t.Errorf("search by column name = %d, want 200", code)
	}
}
