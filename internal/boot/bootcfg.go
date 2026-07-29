// Package boot renders the ESXi loader files a host fetches during install.
//
// It exists to be shared by every transport. The same boot.cfg rewrite was
// previously implemented once in the TFTP server and again in the UEFI HTTP
// handler, and the two had drifted — a fix removing cdromBoot from the kernel
// options was applied only to the HTTP copy, so PXE-booted hosts got a
// different command line to HTTP-booted ones. The transports differ in exactly
// one respect, the prefix the loader fetches modules from, so that is a
// parameter and everything else is shared.
package boot

import (
	"regexp"
	"strconv"
	"strings"
)

// Params is everything a host's boot.cfg depends on.
type Params struct {
	// Prefix is where the loader fetches kernel and modules from. TFTP serves
	// a flat directory, so it is a folder name; HTTP boot needs an absolute
	// URL.
	Prefix string

	// KickstartURL is the ks.cfg the installer fetches once it is running.
	KickstartURL string

	// Mac pins the kickstart request to the interface that booted, so the
	// server can identify the host by source address.
	Mac string

	IP      string
	Netmask string
	Gateway string

	// Vlan is applied only when set.
	Vlan string

	// AllowLegacyCPU permits install on CPUs the release has dropped support
	// for. Both callers set this, matching --forceunsupportedinstall in the
	// kickstart template; the group-level toggle that used to control it was
	// removed from models.GroupOptions. It is a field rather than a constant
	// so the decision is visible and reversible.
	AllowLegacyCPU bool
}

var (
	// Paths are flattened because the loader fetches every file from one
	// directory, whatever the ISO layout was.
	pathSeparator = regexp.MustCompile("/")

	// cdromBoot makes the installer look for its media on a CD-ROM that is
	// not there when booting over the network.
	cdromBoot = regexp.MustCompile("cdromBoot")

	kernelOpt = regexp.MustCompile("kernelopt=.*")
	prefix    = regexp.MustCompile("prefix=")
)

// RenderConfig rewrites an ISO's boot.cfg for one host.
//
// The order matters: separators are stripped before any URL is added, or the
// slashes in those URLs would be stripped too.
func RenderConfig(src []byte, p Params) []byte {
	out := pathSeparator.ReplaceAllLiteral(src, nil)
	out = cdromBoot.ReplaceAllLiteral(out, nil)

	out = appendKernelOpt(out, " ks="+p.KickstartURL)
	out = appendKernelOpt(out, " netdevice="+p.Mac+" ip="+p.IP+" netmask="+p.Netmask+" gateway="+p.Gateway)

	if p.Vlan != "" {
		out = appendKernelOpt(out, " vlanid="+p.Vlan)
	}
	if p.AllowLegacyCPU {
		out = appendKernelOpt(out, " allowLegacyCPU=true")
	}

	// prefix= is empty in the shipped file; the value is appended to it.
	if o := prefix.Find(out); o != nil {
		out = prefix.ReplaceAllLiteral(out, append(o, []byte(p.Prefix)...))
	}

	return out
}

// appendKernelOpt adds to the end of the kernelopt line, leaving the rest of
// the file alone.
func appendKernelOpt(src []byte, s string) []byte {
	o := kernelOpt.Find(src)
	if o == nil {
		return src
	}

	// regexp.Find caps the returned slice's capacity to its length, so this
	// append allocates rather than writing into src.
	return kernelOpt.ReplaceAllLiteral(src, append(o, []byte(s)...))
}

// KickstartURL builds the URL the installer fetches its kickstart from.
func KickstartURL(host string, port int) string {
	return "https://" + host + ":" + strconv.Itoa(port) + "/ks.cfg"
}

// HTTPPrefix builds the prefix for a host booting over HTTP or HTTPS, where
// the loader fetches modules back over the same transport.
func HTTPPrefix(scheme, host string, port int) string {
	return scheme + "://" + host + ":" + strconv.Itoa(port) + "/esx/"
}

// TFTPPrefix builds the prefix for a host booting over TFTP, which fetches
// modules from a directory relative to the TFTP root.
func TFTPPrefix(imagePath string) string {
	// The image directory name, however the path was written. The previous
	// implementation indexed strings.Split(path, "/")[1], which gave the wrong
	// element for a path with a leading "./".
	trimmed := strings.TrimSuffix(imagePath, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}
