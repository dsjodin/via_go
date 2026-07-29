package boot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Shaped like the boot.cfg ESXi ships: an empty prefix, a kernel line, a
// kernelopt carrying cdromBoot, and a modules list of slash-prefixed paths.
const fixture = `bootstate=0
title=Loading ESXi installer
timeout=5
prefix=
kernel=/b.b00
kernelopt=cdromBoot runweasel
modules=/jumpstrt.gz --- /useropts.gz --- /features.gz --- /k.b00
build=
updated=0
`

func params() Params {
	return Params{
		Prefix:         "https://192.168.1.2:8443/esx/",
		KickstartURL:   "https://192.168.1.2:8443/ks.cfg",
		Mac:            "00:0c:29:00:00:01",
		IP:             "192.168.1.50",
		Netmask:        "255.255.255.0",
		Gateway:        "192.168.1.1",
		AllowLegacyCPU: true,
	}
}

func line(t *testing.T, out, key string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, key) {
			return l
		}
	}
	t.Fatalf("no %s line in rendered boot.cfg:\n%s", key, out)
	return ""
}

func TestRenderConfigKernelOpt(t *testing.T) {
	out := string(RenderConfig([]byte(fixture), params()))
	opt := line(t, out, "kernelopt=")

	for _, want := range []string{
		"ks=https://192.168.1.2:8443/ks.cfg",
		"netdevice=00:0c:29:00:00:01",
		"ip=192.168.1.50",
		"netmask=255.255.255.0",
		"gateway=192.168.1.1",
		"allowLegacyCPU=true",
		"runweasel",
	} {
		if !strings.Contains(opt, want) {
			t.Errorf("kernelopt missing %q\ngot: %s", want, opt)
		}
	}

	if strings.Contains(opt, "cdromBoot") {
		t.Errorf("cdromBoot was not removed\ngot: %s", opt)
	}
}

// This is the regression that matters most: the rewrite appends to a subslice
// returned by regexp.Find, and every other line must survive intact. Losing a
// module means the install fails partway through.
func TestRenderConfigPreservesEveryOtherLine(t *testing.T) {
	out := string(RenderConfig([]byte(fixture), params()))

	for _, want := range []string{
		"bootstate=0",
		"title=Loading ESXi installer",
		"timeout=5",
		"kernel=b.b00",
		"build=",
		"updated=0",
		"jumpstrt.gz", "useropts.gz", "features.gz", "k.b00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q\n---\n%s", want, out)
		}
	}

	if got, want := strings.Count(out, "\n"), strings.Count(fixture, "\n"); got != want {
		t.Errorf("line count changed: %d, want %d\n---\n%s", got, want, out)
	}
}

func TestRenderConfigFlattensPaths(t *testing.T) {
	out := string(RenderConfig([]byte(fixture), Params{Prefix: "images/esxi80"}))

	modules := line(t, out, "modules=")
	if strings.Contains(modules, "/") {
		t.Errorf("modules line still has path separators: %s", modules)
	}
	if k := line(t, out, "kernel="); strings.Contains(k, "/") {
		t.Errorf("kernel line still has path separators: %s", k)
	}
}

// Separators must be stripped before URLs are added, or the slashes in those
// URLs are stripped too and the host is told to fetch from "https:192.168.1.2".
func TestRenderConfigDoesNotMangleURLs(t *testing.T) {
	out := string(RenderConfig([]byte(fixture), params()))

	if !strings.Contains(out, "https://192.168.1.2:8443/ks.cfg") {
		t.Errorf("kickstart URL was mangled\n---\n%s", out)
	}
	if !strings.Contains(line(t, out, "prefix="), "https://192.168.1.2:8443/esx/") {
		t.Errorf("prefix URL was mangled\n---\n%s", out)
	}
}

func TestRenderConfigVlan(t *testing.T) {
	p := params()
	p.Vlan = "101"

	if opt := line(t, string(RenderConfig([]byte(fixture), p)), "kernelopt="); !strings.Contains(opt, "vlanid=101") {
		t.Errorf("missing vlanid\ngot: %s", opt)
	}

	p.Vlan = ""
	if opt := line(t, string(RenderConfig([]byte(fixture), p)), "kernelopt="); strings.Contains(opt, "vlanid") {
		t.Errorf("vlanid present with no vlan set\ngot: %s", opt)
	}
}

func TestRenderConfigAllowLegacyCPU(t *testing.T) {
	p := params()
	p.AllowLegacyCPU = false

	if opt := line(t, string(RenderConfig([]byte(fixture), p)), "kernelopt="); strings.Contains(opt, "allowLegacyCPU") {
		t.Errorf("allowLegacyCPU present when not requested\ngot: %s", opt)
	}
}

// A boot.cfg without the lines the rewrite targets must come back unchanged
// rather than panicking or emptying itself.
func TestRenderConfigToleratesMissingLines(t *testing.T) {
	for _, src := range []string{"", "title=nothing to do\n", "prefix=\n"} {
		out := string(RenderConfig([]byte(src), params()))
		if strings.Contains(out, "kernelopt") {
			t.Errorf("invented a kernelopt line for %q: %s", src, out)
		}
	}
}

func TestTFTPPrefix(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"images/esxi80", "esxi80"},
		{"./images/esxi80", "esxi80"},
		{"/var/lib/go-via/images/esxi80", "esxi80"},
		{"images/esxi80/", "esxi80"},
		{"esxi80", "esxi80"},
	} {
		if got := TFTPPrefix(tc.in); got != tc.want {
			t.Errorf("TFTPPrefix(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestURLBuilders(t *testing.T) {
	if got, want := KickstartURL("10.0.0.1", 8443), "https://10.0.0.1:8443/ks.cfg"; got != want {
		t.Errorf("KickstartURL = %q, want %q", got, want)
	}
	if got, want := HTTPPrefix("http", "10.0.0.1", 80), "http://10.0.0.1:80/esx/"; got != want {
		t.Errorf("HTTPPrefix = %q, want %q", got, want)
	}
}

func TestImageLookupAcrossISOLayouts(t *testing.T) {
	layouts := []struct {
		name    string
		mboot   string
		crypto  string
		bootcfg string
	}{
		{"uppercase x86", "EFI/BOOT/BOOTX64.EFI", "EFI/BOOT/CRYPTO64.EFI", "BOOT.CFG"},
		{"lowercase x86", "efi/boot/bootx64.efi", "efi/boot/crypto64.efi", "boot.cfg"},
		{"uppercase arm", "EFI/BOOT/BOOTAA64.EFI", "EFI/BOOT/CRYPTO64.EFI", "BOOT.CFG"},
		{"flat mboot", "MBOOT.EFI", "EFI/BOOT/CRYPTO64.EFI", "BOOT.CFG"},
	}

	for _, l := range layouts {
		t.Run(l.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range []string{l.mboot, l.crypto, l.bootcfg} {
				p := filepath.Join(dir, f)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			}

			if got, err := Mboot(dir); err != nil || !strings.HasSuffix(got, l.mboot) {
				t.Errorf("Mboot = %q, %v; want a path ending %q", got, err, l.mboot)
			}
			if got, err := Crypto64(dir); err != nil || !strings.HasSuffix(got, l.crypto) {
				t.Errorf("Crypto64 = %q, %v; want a path ending %q", got, err, l.crypto)
			}
			if got, err := Config(dir); err != nil || !strings.HasSuffix(got, l.bootcfg) {
				t.Errorf("Config = %q, %v; want a path ending %q", got, err, l.bootcfg)
			}
		})
	}
}

func TestImageLookupReportsMissingFiles(t *testing.T) {
	dir := t.TempDir()

	for _, fn := range []struct {
		name string
		f    func(string) (string, error)
	}{
		{"Mboot", Mboot},
		{"Crypto64", Crypto64},
		{"Config", Config},
	} {
		if got, err := fn.f(dir); err == nil {
			t.Errorf("%s on an empty directory = %q, want an error", fn.name, got)
		}
	}
}
