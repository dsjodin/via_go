package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dsjodin/via_go/internal/config"
	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"gorm.io/datatypes"
)

var dbSeq uint64

// A BOOT.CFG shaped like the ones ESXi ships: a kernel line, a long modules
// line with slash-separated paths, and a prefix the server has to rewrite.
const fixtureBootCfg = `bootstate=0
title=Loading ESXi installer
timeout=5
prefix=
kernel=/b.b00
kernelopt=cdromBoot runweasel
modules=/jumpstrt.gz --- /useropts.gz --- /features.gz --- /k.b00
build=
updated=0
`

func setup(t *testing.T, group model.Group, host model.Host) (model.Host, model.Image, *config.Config) {
	t.Helper()

	dsn := fmt.Sprintf("file:uefitest%d?mode=memory&cache=shared", atomic.AddUint64(&dbSeq, 1))
	if err := store.Open(dsn, false); err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := store.DB.AutoMigrate(&model.Host{}, &model.Group{}, &model.Image{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res := store.DB.Create(&group); res.Error != nil {
		t.Fatalf("create group: %v", res.Error)
	}
	if err := host.GroupID.UnmarshalJSON([]byte(strconv.Itoa(group.ID))); err != nil {
		t.Fatalf("set group id: %v", err)
	}
	host.Group = group
	if res := store.DB.Create(&host); res.Error != nil {
		t.Fatalf("create host: %v", res.Error)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "BOOT.CFG"), []byte(fixtureBootCfg), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	image := model.Image{ImageForm: model.ImageForm{Path: dir}}
	if res := store.DB.Create(&image); res.Error != nil {
		t.Fatalf("create image: %v", res.Error)
	}

	return host, image, &config.Config{Port: 8443}
}

func testGroup() model.Group {
	return model.Group{
		GroupForm: model.GroupForm{
			Name:    "test",
			Netmask: "255.255.255.0",
			Gateway: "192.168.1.1",
			Options: datatypes.JSON(`{}`),
		},
	}
}

func testHost() model.Host {
	return model.Host{
		HostForm: model.HostForm{
			IP:       "192.168.1.50",
			Mac:      "00:0c:29:00:00:01",
			Hostname: "esx01",
		},
	}
}

func render(t *testing.T, group model.Group, host model.Host) string {
	t.Helper()
	h, image, conf := setup(t, group, host)

	out, err := serveBootCfg("/boot.cfg", h, image, conf, "192.168.1.2", "192.168.1.50")
	if err != nil {
		t.Fatalf("serveBootCfg: %v", err)
	}
	return string(out)
}

func kernelopt(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "kernelopt=") {
			return line
		}
	}
	t.Fatalf("no kernelopt line in rendered boot.cfg:\n%s", out)
	return ""
}

func TestBootCfgKernelOpt(t *testing.T) {
	out := render(t, testGroup(), testHost())
	opt := kernelopt(t, out)

	for _, want := range []string{
		"ks=https://192.168.1.2:8443/ks.cfg",
		"netdevice=00:0c:29:00:00:01",
		"ip=192.168.1.50",
		"netmask=255.255.255.0",
		"gateway=192.168.1.1",
	} {
		if !strings.Contains(opt, want) {
			t.Errorf("kernelopt missing %q\ngot: %s", want, opt)
		}
	}

	// cdromBoot must go, or the installer looks for the media on a CD-ROM
	// that is not there.
	if strings.Contains(opt, "cdromBoot") {
		t.Errorf("cdromBoot was not removed\ngot: %s", opt)
	}
}

// Every other line of the file has to survive intact. The rewrite works by
// appending to a subslice returned by regexp.Find, which can write into the
// source buffer in place and clobber whatever follows the match.
func TestBootCfgPreservesOtherLines(t *testing.T) {
	out := render(t, testGroup(), testHost())

	for _, want := range []string{
		"bootstate=0",
		"title=Loading ESXi installer",
		"timeout=5",
		"kernel=b.b00",
		"updated=0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered boot.cfg lost %q\n---\n%s", want, out)
		}
	}

	// The modules list is what the loader fetches; losing an entry means the
	// install fails partway through with nothing useful logged.
	for _, mod := range []string{"jumpstrt.gz", "useropts.gz", "features.gz", "k.b00"} {
		if !strings.Contains(out, mod) {
			t.Errorf("rendered boot.cfg lost module %q\n---\n%s", mod, out)
		}
	}

	if n := strings.Count(out, "\n"); n < 9 {
		t.Errorf("rendered boot.cfg has %d newlines, want at least 9 — lines were clobbered\n---\n%s", n, out)
	}
}

// Paths are flattened because the files are served from a single directory.
func TestBootCfgStripsPathSeparators(t *testing.T) {
	out := render(t, testGroup(), testHost())

	modules := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "modules=") {
			modules = line
		}
	}
	if modules == "" {
		t.Fatalf("no modules line\n---\n%s", out)
	}
	if strings.Contains(modules, "/") {
		t.Errorf("modules line still contains path separators: %s", modules)
	}
}

func TestBootCfgPrefixPointsAtTheServer(t *testing.T) {
	out := render(t, testGroup(), testHost())

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "prefix=") {
			if !strings.Contains(line, "192.168.1.2:8443") {
				t.Errorf("prefix does not point at the server: %s", line)
			}
			return
		}
	}
	t.Errorf("no prefix line in rendered boot.cfg\n---\n%s", out)
}

func TestBootCfgVlan(t *testing.T) {
	group := testGroup()
	group.Vlan = "101"
	host := testHost()

	opt := kernelopt(t, render(t, group, host))
	if !strings.Contains(opt, "vlanid=101") {
		t.Errorf("kernelopt missing vlanid\ngot: %s", opt)
	}
}

func TestBootCfgNoVlan(t *testing.T) {
	opt := kernelopt(t, render(t, testGroup(), testHost()))
	if strings.Contains(opt, "vlanid") {
		t.Errorf("kernelopt has a vlanid for a group with no vlan\ngot: %s", opt)
	}
}
