package netutil

import "testing"

func TestNetmaskToCIDR(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"255.255.255.0", 24},
		{"255.255.254.0", 23},
		{"255.255.0.0", 16},
		{"255.255.255.252", 30},
		{"255.255.255.255", 32},
		{"0.0.0.0", 0},
		// CIDR forms, with and without the slash, and with stray whitespace.
		{"24", 24},
		{"/24", 24},
		{" /24 ", 24},
		{"0", 0},
		{"32", 32},
	}

	for _, tc := range tests {
		got, err := NetmaskToCIDR(tc.in)
		if err != nil {
			t.Errorf("NetmaskToCIDR(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NetmaskToCIDR(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestNetmaskToCIDRRejectsInvalid(t *testing.T) {
	for _, in := range []string{
		"",
		"not-a-mask",
		"33",
		"-1",
		"/33",
		"255.255.255",
		"2001:db8::1",
		// Non-contiguous masks have no CIDR length.
		"255.0.255.0",
		"255.255.0.255",
	} {
		if got, err := NetmaskToCIDR(in); err == nil {
			t.Errorf("NetmaskToCIDR(%q) = %d, want an error", in, got)
		}
	}
}

func TestNetworkAddress(t *testing.T) {
	tests := []struct {
		gateway string
		netmask string
		want    string
	}{
		{"192.168.1.1", "255.255.255.0", "192.168.1.0"},
		{"192.168.1.1", "24", "192.168.1.0"},
		{"192.168.1.1", "/24", "192.168.1.0"},
		{"10.20.30.40", "255.255.0.0", "10.20.0.0"},
		{"172.16.5.9", "255.255.255.240", "172.16.5.0"},
		{"10.0.0.5", "255.255.255.252", "10.0.0.4"},
		{"10.0.0.1", "0.0.0.0", "0.0.0.0"},
		{"10.0.0.1", "32", "10.0.0.1"},
	}

	for _, tc := range tests {
		got, err := NetworkAddress(tc.gateway, tc.netmask)
		if err != nil {
			t.Errorf("NetworkAddress(%q, %q): unexpected error: %v", tc.gateway, tc.netmask, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NetworkAddress(%q, %q) = %s, want %s", tc.gateway, tc.netmask, got, tc.want)
		}
	}
}

func TestBroadcastAddress(t *testing.T) {
	tests := []struct {
		gateway string
		netmask string
		want    string
	}{
		{"192.168.1.1", "255.255.255.0", "192.168.1.255"},
		{"192.168.1.1", "24", "192.168.1.255"},
		{"10.20.30.40", "255.255.0.0", "10.20.255.255"},
		{"172.16.5.9", "255.255.255.240", "172.16.5.15"},
		{"10.0.0.5", "255.255.255.252", "10.0.0.7"},
		{"10.0.0.1", "32", "10.0.0.1"},
		{"10.0.0.1", "0.0.0.0", "255.255.255.255"},
	}

	for _, tc := range tests {
		got, err := BroadcastAddress(tc.gateway, tc.netmask)
		if err != nil {
			t.Errorf("BroadcastAddress(%q, %q): unexpected error: %v", tc.gateway, tc.netmask, err)
			continue
		}
		if got != tc.want {
			t.Errorf("BroadcastAddress(%q, %q) = %s, want %s", tc.gateway, tc.netmask, got, tc.want)
		}
	}
}

// A group with a blank or malformed gateway is entirely plausible — the field
// is not validated on the way in — and must produce an error rather than a
// panic or a nonsense address.
func TestAddressHelpersRejectBadInput(t *testing.T) {
	bad := []struct {
		gateway string
		netmask string
	}{
		{"", "255.255.255.0"},
		{"not-an-ip", "255.255.255.0"},
		{"2001:db8::1", "255.255.255.0"},
		{"192.168.1.1", ""},
		{"192.168.1.1", "not-a-mask"},
		{"192.168.1.1", "33"},
		{"192.168.1.1", "2001:db8::1"},
	}

	for _, tc := range bad {
		if got, err := NetworkAddress(tc.gateway, tc.netmask); err == nil {
			t.Errorf("NetworkAddress(%q, %q) = %s, want an error", tc.gateway, tc.netmask, got)
		}
		if got, err := BroadcastAddress(tc.gateway, tc.netmask); err == nil {
			t.Errorf("BroadcastAddress(%q, %q) = %s, want an error", tc.gateway, tc.netmask, got)
		}
	}
}
