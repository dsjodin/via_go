package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dsjodin/via_go/db"
	"github.com/dsjodin/via_go/models"
	"github.com/dsjodin/via_go/secrets"
	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

var dbSeq uint64

func init() { gin.SetMode(gin.TestMode) }

// seed builds an in-memory database holding one host in one group, and returns
// the host.
func seed(t *testing.T, group models.Group, host models.Host) *models.Host {
	t.Helper()

	dsn := fmt.Sprintf("file:apitest%d?mode=memory&cache=shared", atomic.AddUint64(&dbSeq, 1))
	if err := db.Open(dsn, false); err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.DB.AutoMigrate(&models.Host{}, &models.Option{}, &models.DeviceClass{}, &models.Group{}, &models.Image{}, &models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	encrypted, err := secrets.Encrypt(group.Password, testKey)
	if err != nil {
		t.Fatalf("encrypt password: %v", err)
	}
	group.Password = encrypted

	if res := db.DB.Create(&group); res.Error != nil {
		t.Fatalf("create group: %v", res.Error)
	}
	if err := host.GroupID.UnmarshalJSON([]byte(strconv.Itoa(group.ID))); err != nil {
		t.Fatalf("set group id: %v", err)
	}
	if res := db.DB.Create(&host); res.Error != nil {
		t.Fatalf("create host: %v", res.Error)
	}

	return &host
}

// render performs a ks.cfg request as if it came from the host's address.
func render(t *testing.T, hostIP string) *httptest.ResponseRecorder {
	t.Helper()

	r := gin.New()
	r.GET("/ks.cfg", Ks(testKey))

	req := httptest.NewRequest(http.MethodGet, "/ks.cfg", nil)
	req.RemoteAddr = hostIP + ":54321"

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func defaultGroup() models.Group {
	return models.Group{
		GroupForm: models.GroupForm{
			Name:     "test",
			Netmask:  "255.255.255.0",
			Gateway:  "192.168.1.1",
			DNS:      "10.0.0.53",
			NTP:      "10.0.0.123",
			Password: "VMware1!",
			Options:  datatypes.JSON(`{}`),
		},
	}
}

func defaultHost() models.Host {
	return models.Host{
		HostForm: models.HostForm{
			IP:       "192.168.1.50",
			Mac:      "00:0c:29:00:00:01",
			Hostname: "esx01",
			Domain:   "example.com",
			Reimage:  true,
		},
	}
}

func TestKsRendersHostNetworking(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())

	body := render(t, "192.168.1.50").Body.String()

	for _, want := range []string{
		"--ip=192.168.1.50",
		"--gateway=192.168.1.1",
		"--netmask=255.255.255.0",
		"--nameserver=10.0.0.53",
		"--hostname=esx01",
		"--device=00:0c:29:00:00:01",
		"esxcli system hostname set --fqdn esx01.example.com",
		"--server 10.0.0.123",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered ks.cfg is missing %q\n---\n%s", want, body)
		}
	}
}

// The kickstart carries the ESXi root password in the clear, which is the
// point of storing it encrypted at rest. If the round trip broke, the install
// would silently come up with the wrong password.
func TestKsRendersTheDecryptedRootPassword(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())

	body := render(t, "192.168.1.50").Body.String()

	if !strings.Contains(body, "rootpw VMware1!") {
		t.Errorf("rendered ks.cfg does not carry the decrypted root password\n---\n%s", body)
	}
}

// Re-imaging must be cleared as the kickstart is served, or the host boots
// into the installer again on every reboot.
func TestKsClearsTheReimageFlag(t *testing.T) {
	host := seed(t, defaultGroup(), defaultHost())

	if rec := render(t, "192.168.1.50"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var after models.Host
	if res := db.DB.First(&after, host.ID); res.Error != nil {
		t.Fatalf("reload host: %v", res.Error)
	}
	if after.Reimage {
		t.Error("reimage is still set after serving the kickstart; the host will reinstall in a loop")
	}
	if after.Progress != 50 {
		t.Errorf("progress = %d, want 50", after.Progress)
	}
}

func TestKsGroupOptions(t *testing.T) {
	tests := []struct {
		name    string
		options string
		want    []string
		absent  []string
	}{
		{
			name:    "erase disks",
			options: `{"erasedisks":true}`,
			want:    []string{"clearpart --overwritevmfs --alldrives"},
		},
		{
			name:    "no erase disks",
			options: `{"erasedisks":false}`,
			absent:  []string{"clearpart"},
		},
		{
			name:    "ssh",
			options: `{"ssh":true}`,
			want:    []string{"vim-cmd hostsvc/enable_ssh", "vim-cmd hostsvc/start_ssh"},
		},
		{
			name:    "no ssh",
			options: `{"ssh":false}`,
			absent:  []string{"enable_ssh"},
		},
		{
			name:    "create vmfs",
			options: `{"createvmfs":true}`,
			absent:  []string{"--novmfsondisk"},
		},
		{
			name:    "no create vmfs",
			options: `{"createvmfs":false}`,
			want:    []string{"--novmfsondisk"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			group := defaultGroup()
			group.Options = datatypes.JSON(tc.options)
			seed(t, group, defaultHost())

			body := render(t, "192.168.1.50").Body.String()

			for _, want := range tc.want {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q\n---\n%s", want, body)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(body, absent) {
					t.Errorf("unexpectedly contains %q\n---\n%s", absent, body)
				}
			}
		})
	}
}

func TestKsBootDiskAndVlan(t *testing.T) {
	group := defaultGroup()
	group.BootDisk = "mpx.vmhba0:C0:T0:L0"
	group.Vlan = "101"
	seed(t, group, defaultHost())

	body := render(t, "192.168.1.50").Body.String()

	for _, want := range []string{
		"install --disk=/vmfs/devices/disks/mpx.vmhba0:C0:T0:L0",
		"--vlanid=101",
		"esxcli network vswitch standard portgroup set --vlan-id 101",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q\n---\n%s", want, body)
		}
	}
}

func TestKsSyslog(t *testing.T) {
	group := defaultGroup()
	group.Syslog = "udp://10.0.0.99:514"
	seed(t, group, defaultHost())

	body := render(t, "192.168.1.50").Body.String()

	if !strings.Contains(body, "esxcli system syslog config set --loghost=udp://10.0.0.99:514") {
		t.Errorf("missing syslog configuration\n---\n%s", body)
	}
}

// A group-level override replaces the built-in template, and a host-level
// override wins over the group.
func TestKsOverrides(t *testing.T) {
	enc := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

	t.Run("group override", func(t *testing.T) {
		group := defaultGroup()
		group.Ks = enc("# group template for {{ .hostname }}")
		seed(t, group, defaultHost())

		body := render(t, "192.168.1.50").Body.String()
		if !strings.Contains(body, "# group template for esx01") {
			t.Errorf("group override was not used\n---\n%s", body)
		}
		if strings.Contains(body, "vmaccepteula") {
			t.Errorf("default template leaked into an overridden kickstart\n---\n%s", body)
		}
	})

	t.Run("host override wins", func(t *testing.T) {
		group := defaultGroup()
		group.Ks = enc("# group template")
		host := defaultHost()
		host.Ks = enc("# host template for {{ .hostname }}")
		seed(t, group, host)

		body := render(t, "192.168.1.50").Body.String()
		if !strings.Contains(body, "# host template for esx01") {
			t.Errorf("host override was not used\n---\n%s", body)
		}
		if strings.Contains(body, "# group template") {
			t.Errorf("group override was used despite a host override\n---\n%s", body)
		}
	})
}

// A request from an address with no host record must not be served a
// kickstart — it carries the root password.
func TestKsRejectsUnknownRequester(t *testing.T) {
	seed(t, defaultGroup(), defaultHost())

	rec := render(t, "192.168.1.99")

	if rec.Code == http.StatusOK {
		t.Errorf("served a kickstart to an unknown address\n---\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "VMware1!") {
		t.Error("leaked the root password to an unknown address")
	}
}
