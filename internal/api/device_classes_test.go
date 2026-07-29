package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
)

// device classes are the simplest resource — pure CRUD with no custom logic —
// so they are the reference for what the generic handlers must preserve.
func deviceClassRouter() *gin.Engine {
	r := gin.New()
	r.GET("/v1/device_classes", ListDeviceClasses)
	r.GET("/v1/device_classes/:id", GetDeviceClass)
	r.POST("/v1/device_classes/search", SearchDeviceClass)
	r.POST("/v1/device_classes", CreateDeviceClass)
	r.PATCH("/v1/device_classes/:id", UpdateDeviceClass)
	r.DELETE("/v1/device_classes/:id", DeleteDeviceClass)
	return r
}

func seedDeviceClasses(t *testing.T) {
	t.Helper()
	seed(t, defaultGroup(), defaultHost())

	for _, dc := range []model.DeviceClass{
		{DeviceClassForm: model.DeviceClassForm{Name: "PXE-UEFI_x64", VendorClass: "PXEClient:Arch:00007"}},
		{DeviceClassForm: model.DeviceClassForm{Name: "HTTP-UEFI_x64", VendorClass: "HTTPClient:Arch:00016"}},
	} {
		if res := store.DB.Create(&dc); res.Error != nil {
			t.Fatalf("seed device class: %v", res.Error)
		}
	}
}

func TestDeviceClassList(t *testing.T) {
	seedDeviceClasses(t)

	rec := do(t, deviceClassRouter(), http.MethodGet, "/v1/device_classes", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var items []model.DeviceClass
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d device classes, want 2", len(items))
	}
}

func TestDeviceClassGet(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	rec := do(t, r, http.MethodGet, "/v1/device_classes/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var item model.DeviceClass
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.Name != "PXE-UEFI_x64" {
		t.Errorf("name = %q, want PXE-UEFI_x64", item.Name)
	}
}

func TestDeviceClassGetErrors(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	if code := do(t, r, http.MethodGet, "/v1/device_classes/999", "").Code; code != http.StatusNotFound {
		t.Errorf("missing id = %d, want 404", code)
	}
	if code := do(t, r, http.MethodGet, "/v1/device_classes/abc", "").Code; code != http.StatusBadRequest {
		t.Errorf("non-numeric id = %d, want 400", code)
	}
}

func TestDeviceClassCreate(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	rec := do(t, r, http.MethodPost, "/v1/device_classes",
		`{"name":"PXE-UEFI_ARM64","vendor_class":"PXEClient:Arch:00011"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var created model.DeviceClass
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == 0 {
		t.Error("created device class has no id")
	}

	var stored model.DeviceClass
	if res := store.DB.First(&stored, created.ID); res.Error != nil {
		t.Fatalf("reload: %v", res.Error)
	}
	if stored.VendorClass != "PXEClient:Arch:00011" {
		t.Errorf("vendor class = %q, want PXEClient:Arch:00011", stored.VendorClass)
	}
}

func TestDeviceClassUpdate(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	rec := do(t, r, http.MethodPatch, "/v1/device_classes/1", `{"name":"renamed"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var stored model.DeviceClass
	store.DB.First(&stored, 1)
	if stored.Name != "renamed" {
		t.Errorf("name = %q, want renamed", stored.Name)
	}
	// A partial update must not clear the fields it does not mention.
	if stored.VendorClass != "PXEClient:Arch:00007" {
		t.Errorf("vendor class = %q, want it left alone", stored.VendorClass)
	}
}

func TestDeviceClassUpdateErrors(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	if code := do(t, r, http.MethodPatch, "/v1/device_classes/999", `{"name":"x"}`).Code; code != http.StatusNotFound {
		t.Errorf("missing id = %d, want 404", code)
	}
	if code := do(t, r, http.MethodPatch, "/v1/device_classes/abc", `{"name":"x"}`).Code; code != http.StatusBadRequest {
		t.Errorf("non-numeric id = %d, want 400", code)
	}
}

func TestDeviceClassDelete(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	if code := do(t, r, http.MethodDelete, "/v1/device_classes/1", "").Code; code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", code)
	}

	var stored model.DeviceClass
	if res := store.DB.First(&stored, 1); res.Error == nil {
		t.Error("device class still present after delete")
	}

	if code := do(t, r, http.MethodDelete, "/v1/device_classes/999", "").Code; code != http.StatusNotFound {
		t.Errorf("missing id = %d, want 404", code)
	}
}

func TestDeviceClassSearch(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	rec := do(t, r, http.MethodPost, "/v1/device_classes/search", `{"name":"HTTP-UEFI_x64"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	var item model.DeviceClass
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if item.VendorClass != "HTTPClient:Arch:00016" {
		t.Errorf("found %q, want the HTTP device class", item.VendorClass)
	}
}

// Search keys are column names, not SQL. Before this was fixed, the key went
// straight into a condition string: {"name = ? OR 1=1": "nope"} matched every
// row, and a subquery in the key could read any column of any table —
// including the encrypted ESXi root passwords on groups.
func TestDeviceClassSearchRejectsSQLInKeys(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	for _, body := range []string{
		`{"name = ? OR 1=1":"nope"}`,
		`{"1=1 OR name = ?":"nope"}`,
		`{"vendor_class IN (SELECT password FROM groups) OR name = ?":"nope"}`,
	} {
		rec := do(t, r, http.MethodPost, "/v1/device_classes/search", body)
		if rec.Code == http.StatusOK {
			t.Errorf("search executed SQL supplied in a key: %s -> %s", body, rec.Body.String())
		}
	}
}

func TestDeviceClassSearchNoMatch(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	if code := do(t, r, http.MethodPost, "/v1/device_classes/search", `{"name":"absent"}`).Code; code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a search that matches nothing", code)
	}
}

func TestDeviceClassSearchRequiresFields(t *testing.T) {
	seedDeviceClasses(t)
	r := deviceClassRouter()

	if code := do(t, r, http.MethodPost, "/v1/device_classes/search", `{}`).Code; code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an empty search", code)
	}
}
