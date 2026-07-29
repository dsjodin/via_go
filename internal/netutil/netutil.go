// Package netutil holds the IPv4 address arithmetic shared by the DHCP server
// and the API.
//
// NetmaskToCIDR and NetworkAddress previously existed as byte-identical copies
// in both api/hosts.go and dhcpd/dhcpd.go, which is how the boot.cfg rewrite
// came to diverge as well. One copy, one place.
package netutil

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// NetworkAddress returns the IPv4 network address for the provided gateway IP
// and netmask. netmask may be either a CIDR length ("24" or "/24") or dotted
// decimal ("255.255.255.0"). Returned network is the base network address
// (e.g. "192.168.1.0").
// Returns an error for invalid input or non-IPv4 addresses.
func NetworkAddress(gateway string, netmask string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(gateway)).To4()
	if ip == nil {
		return "", errors.New("invalid IPv4 gateway")
	}

	m := strings.TrimSpace(netmask)

	// accept "/24" or "24"
	m = strings.TrimPrefix(m, "/")

	// If netmask is numeric (CIDR bits)
	if bits, err := strconv.Atoi(m); err == nil {
		if bits < 0 || bits > 32 {
			return "", errors.New("invalid CIDR mask length")
		}
		mask := net.CIDRMask(bits, 32)
		network := ip.Mask(mask)
		return network.String(), nil
	}

	// Otherwise expect dotted decimal like "255.255.255.0"
	pm := net.ParseIP(m)
	if pm == nil {
		return "", errors.New("invalid netmask format")
	}
	pm4 := pm.To4()
	if pm4 == nil {
		return "", errors.New("invalid IPv4 netmask")
	}
	mask := net.IPMask(pm4)
	network := ip.Mask(mask)
	return network.String(), nil
}

// NetmaskToCIDR converts a netmask in dotted-decimal ("255.255.255.0"),
// or CIDR formats ("24" or "/24") to the CIDR prefix length (e.g. 24).
func NetmaskToCIDR(maskStr string) (int, error) {
	m := strings.TrimSpace(maskStr)
	m = strings.TrimPrefix(m, "/")

	// If already numeric (CIDR)
	if bits, err := strconv.Atoi(m); err == nil {
		if bits < 0 || bits > 32 {
			return 0, fmt.Errorf("invalid CIDR length: %d", bits)
		}
		return bits, nil
	}

	// Expect dotted decimal
	ip := net.ParseIP(m)
	if ip == nil {
		return 0, fmt.Errorf("invalid netmask: %q", maskStr)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return 0, fmt.Errorf("invalid IPv4 netmask: %q", maskStr)
	}
	ones, bits := net.IPMask(ip4).Size()
	if bits != 32 {
		return 0, fmt.Errorf("unexpected mask size: %d", bits)
	}
	return ones, nil
}

// BroadcastAddress returns the IPv4 broadcast address for the provided
// gateway IP and netmask. netmask accepts the same forms as NetworkAddress.
func BroadcastAddress(gateway string, netmask string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(gateway)).To4()
	if ip == nil {
		return "", errors.New("invalid IPv4 gateway")
	}

	m := strings.TrimSpace(netmask)
	m = strings.TrimPrefix(m, "/")

	var mask net.IPMask
	if bits, err := strconv.Atoi(m); err == nil {
		if bits < 0 || bits > 32 {
			return "", errors.New("invalid CIDR mask length")
		}
		mask = net.CIDRMask(bits, 32)
	} else {
		pm := net.ParseIP(m)
		if pm == nil {
			return "", errors.New("invalid netmask format")
		}
		pm4 := pm.To4()
		if pm4 == nil {
			return "", errors.New("invalid IPv4 netmask")
		}
		mask = net.IPMask(pm4)
	}

	network := ip.Mask(mask)
	bcast := make(net.IP, net.IPv4len)
	for i := 0; i < net.IPv4len; i++ {
		bcast[i] = network[i] | ^mask[i]
	}
	return bcast.String(), nil
}
