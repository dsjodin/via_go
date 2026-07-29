package models

import (
	"testing"

	"github.com/google/gopacket/layers"
)

func opt(code layers.DHCPOpt, data string) Option {
	return Option{OptionForm: OptionForm{OpCode: byte(code), Data: data}}
}

func TestToDHCPOptionString(t *testing.T) {
	tests := []struct {
		code layers.DHCPOpt
		data string
	}{
		{layers.DHCPOptHostname, "esx01"},
		{layers.DHCPOptDomainName, "example.com"},
		{layers.DHCPOptRootPath, "/vmfs/volumes"},
		{layers.DHCPOptDomainSearch, "example.com"},
		{66, "10.0.0.1"},  // TFTP server name
		{67, "mboot.efi"}, // TFTP file name
		{67, ""},          // empty is legal, if useless
		{layers.DHCPOptMessage, "a longer message with spaces"},
	}

	for _, tc := range tests {
		got, merge, err := opt(tc.code, tc.data).ToDHCPOption()
		if err != nil {
			t.Errorf("option %d: unexpected error: %v", tc.code, err)
			continue
		}
		if merge {
			t.Errorf("option %d: merge = true, want false", tc.code)
		}
		if string(got.Data) != tc.data {
			t.Errorf("option %d: data = %q, want %q", tc.code, got.Data, tc.data)
		}
		if int(got.Length) != len(tc.data) {
			t.Errorf("option %d: length = %d, want %d", tc.code, got.Length, len(tc.data))
		}
	}
}

// RFC 2132 defines these as a single 4-octet address. net.ParseIP returns the
// 16-byte IPv4-in-IPv6 form for a dotted quad, so encoding its result directly
// produces a 16-byte option carrying a v4-mapped address — not what any client
// expects to read as a subnet mask.
func TestToDHCPOptionSingleIPIsFourBytes(t *testing.T) {
	tests := []struct {
		code layers.DHCPOpt
		data string
		want []byte
	}{
		{layers.DHCPOptSubnetMask, "255.255.255.0", []byte{255, 255, 255, 0}},
		{layers.DHCPOptSubnetMask, "255.255.254.0", []byte{255, 255, 254, 0}},
		{layers.DHCPOptBroadcastAddr, "192.168.1.255", []byte{192, 168, 1, 255}},
		{layers.DHCPOptSolicitAddr, "10.0.0.1", []byte{10, 0, 0, 1}},
	}

	for _, tc := range tests {
		got, merge, err := opt(tc.code, tc.data).ToDHCPOption()
		if err != nil {
			t.Errorf("option %d (%s): unexpected error: %v", tc.code, tc.data, err)
			continue
		}
		if merge {
			t.Errorf("option %d: merge = true, want false", tc.code)
		}
		if got.Length != 4 {
			t.Errorf("option %d (%s): length = %d, want 4", tc.code, tc.data, got.Length)
		}
		if string(got.Data) != string(tc.want) {
			t.Errorf("option %d (%s): data = %v, want %v", tc.code, tc.data, got.Data, tc.want)
		}
	}
}

func TestToDHCPOptionMultiIP(t *testing.T) {
	tests := []struct {
		code layers.DHCPOpt
		data string
		want []byte
	}{
		{layers.DHCPOptRouter, "192.168.1.1", []byte{192, 168, 1, 1}},
		{layers.DHCPOptDNS, "8.8.8.8", []byte{8, 8, 8, 8}},
		{layers.DHCPOptNTPServers, "10.1.2.3", []byte{10, 1, 2, 3}},
		{layers.DHCPOptLogServer, "172.16.0.1", []byte{172, 16, 0, 1}},
	}

	for _, tc := range tests {
		got, merge, err := opt(tc.code, tc.data).ToDHCPOption()
		if err != nil {
			t.Errorf("option %d: unexpected error: %v", tc.code, err)
			continue
		}
		if !merge {
			t.Errorf("option %d: merge = false, want true", tc.code)
		}
		if got.Length != 4 {
			t.Errorf("option %d: length = %d, want 4", tc.code, got.Length)
		}
		if string(got.Data) != string(tc.want) {
			t.Errorf("option %d: data = %v, want %v", tc.code, got.Data, tc.want)
		}
	}
}

// An address that does not parse must be reported. Encoding it as a
// zero-length option hands the client a malformed answer and gives the
// operator nothing to debug.
func TestToDHCPOptionRejectsInvalidIP(t *testing.T) {
	for _, tc := range []struct {
		code layers.DHCPOpt
		data string
	}{
		{layers.DHCPOptSubnetMask, "not-an-ip"},
		{layers.DHCPOptSubnetMask, ""},
		{layers.DHCPOptBroadcastAddr, "999.1.1.1"},
		{layers.DHCPOptRouter, "garbage"},
		{layers.DHCPOptDNS, ""},
		// IPv6 has no representation in these options.
		{layers.DHCPOptRouter, "2001:db8::1"},
		{layers.DHCPOptSubnetMask, "2001:db8::1"},
	} {
		got, _, err := opt(tc.code, tc.data).ToDHCPOption()
		if err == nil {
			t.Errorf("option %d (%q): expected an error, got data %v", tc.code, tc.data, got.Data)
		}
	}
}

func TestToDHCPOptionUint16(t *testing.T) {
	for _, tc := range []struct {
		code layers.DHCPOpt
		data string
		want []byte
	}{
		{layers.DHCPOptInterfaceMTU, "1500", []byte{0x05, 0xdc}},
		{layers.DHCPOptInterfaceMTU, "9000", []byte{0x23, 0x28}},
		{layers.DHCPOptMaxMessageSize, "576", []byte{0x02, 0x40}},
		{layers.DHCPOptBootfileSize, "1", []byte{0x00, 0x01}},
	} {
		got, merge, err := opt(tc.code, tc.data).ToDHCPOption()
		if err != nil {
			t.Errorf("option %d: unexpected error: %v", tc.code, err)
			continue
		}
		if merge {
			t.Errorf("option %d: merge = true, want false", tc.code)
		}
		if string(got.Data) != string(tc.want) {
			t.Errorf("option %d (%s): data = %v, want %v", tc.code, tc.data, got.Data, tc.want)
		}
	}
}

func TestToDHCPOptionUint32(t *testing.T) {
	for _, tc := range []struct {
		code layers.DHCPOpt
		data string
		want []byte
	}{
		{layers.DHCPOptLeaseTime, "3600", []byte{0x00, 0x00, 0x0e, 0x10}},
		{layers.DHCPOptT1, "1800", []byte{0x00, 0x00, 0x07, 0x08}},
		{layers.DHCPOptT2, "3150", []byte{0x00, 0x00, 0x0c, 0x4e}},
		{layers.DHCPOptARPTimeout, "60", []byte{0x00, 0x00, 0x00, 0x3c}},
	} {
		got, merge, err := opt(tc.code, tc.data).ToDHCPOption()
		if err != nil {
			t.Errorf("option %d: unexpected error: %v", tc.code, err)
			continue
		}
		if merge {
			t.Errorf("option %d: merge = true, want false", tc.code)
		}
		if got.Length != 4 {
			t.Errorf("option %d: length = %d, want 4", tc.code, got.Length)
		}
		if string(got.Data) != string(tc.want) {
			t.Errorf("option %d (%s): data = %v, want %v", tc.code, tc.data, got.Data, tc.want)
		}
	}
}

// Option 2 is a signed offset from UTC, so western timezones must round trip
// as two's complement rather than being clamped at zero.
func TestToDHCPOptionInt32SignedTimeOffset(t *testing.T) {
	for _, tc := range []struct {
		data string
		want []byte
	}{
		{"3600", []byte{0x00, 0x00, 0x0e, 0x10}},
		{"0", []byte{0x00, 0x00, 0x00, 0x00}},
		{"-18000", []byte{0xff, 0xff, 0xb9, 0xb0}}, // UTC-5
		{"-3600", []byte{0xff, 0xff, 0xf1, 0xf0}},  // UTC-1
	} {
		got, _, err := opt(layers.DHCPOptTimeOffset, tc.data).ToDHCPOption()
		if err != nil {
			t.Errorf("time offset %s: unexpected error: %v", tc.data, err)
			continue
		}
		if string(got.Data) != string(tc.want) {
			t.Errorf("time offset %s: data = %v, want %v", tc.data, got.Data, tc.want)
		}
	}
}

func TestToDHCPOptionRejectsNonNumeric(t *testing.T) {
	for _, code := range []layers.DHCPOpt{
		layers.DHCPOptInterfaceMTU,
		layers.DHCPOptLeaseTime,
		layers.DHCPOptTimeOffset,
		layers.DHCPOptPathPlateuTableOption,
	} {
		if _, _, err := opt(code, "not-a-number").ToDHCPOption(); err == nil {
			t.Errorf("option %d: expected an error for non-numeric data", code)
		}
	}
}

func TestToDHCPOptionRejectsUnsupportedOpcode(t *testing.T) {
	// 252 is the WPAD option, which this encoder does not know about.
	if _, _, err := opt(252, "http://example.com/wpad.dat").ToDHCPOption(); err == nil {
		t.Error("expected an error for an unsupported opcode")
	}
}

func TestToDHCPOptionMergeFlag(t *testing.T) {
	// Only the repeatable option types may be merged; merging a scalar would
	// concatenate two values into one malformed option.
	mergeable := map[layers.DHCPOpt]bool{
		layers.DHCPOptRouter:                true,
		layers.DHCPOptDNS:                   true,
		layers.DHCPOptNTPServers:            true,
		layers.DHCPOptPathPlateuTableOption: true,
		layers.DHCPOptSubnetMask:            false,
		layers.DHCPOptHostname:              false,
		layers.DHCPOptLeaseTime:             false,
		layers.DHCPOptInterfaceMTU:          false,
		layers.DHCPOptTimeOffset:            false,
	}

	data := map[layers.DHCPOpt]string{
		layers.DHCPOptRouter:                "10.0.0.1",
		layers.DHCPOptDNS:                   "10.0.0.2",
		layers.DHCPOptNTPServers:            "10.0.0.3",
		layers.DHCPOptPathPlateuTableOption: "1500",
		layers.DHCPOptSubnetMask:            "255.255.255.0",
		layers.DHCPOptHostname:              "esx01",
		layers.DHCPOptLeaseTime:             "3600",
		layers.DHCPOptInterfaceMTU:          "1500",
		layers.DHCPOptTimeOffset:            "0",
	}

	for code, want := range mergeable {
		_, merge, err := opt(code, data[code]).ToDHCPOption()
		if err != nil {
			t.Errorf("option %d: unexpected error: %v", code, err)
			continue
		}
		if merge != want {
			t.Errorf("option %d: merge = %v, want %v", code, merge, want)
		}
	}
}

func TestOptionLevelPrecedence(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
		want int
	}{
		{"global", Option{}, 0},
		{"host", Option{OptionForm: OptionForm{HostID: 1}}, 2},
		{"device class", Option{OptionForm: OptionForm{DeviceClassID: 1}}, 3},
		{"host and device class", Option{OptionForm: OptionForm{HostID: 1, DeviceClassID: 1}}, 5},
	}

	for _, tc := range tests {
		if got := tc.opt.Level(); got != tc.want {
			t.Errorf("%s: Level() = %d, want %d", tc.name, got, tc.want)
		}
	}

	// The more specific an option is, the higher it must sort, because
	// AddOptions keeps only the highest level per opcode.
	global := Option{}
	host := Option{OptionForm: OptionForm{HostID: 1}}
	class := Option{OptionForm: OptionForm{DeviceClassID: 1}}
	both := Option{OptionForm: OptionForm{HostID: 1, DeviceClassID: 1}}

	ordered := []Option{global, host, class, both}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1].Level() >= ordered[i].Level() {
			t.Errorf("levels are not strictly increasing in specificity: global=%d host=%d class=%d both=%d",
				global.Level(), host.Level(), class.Level(), both.Level())
			break
		}
	}
}
