package dhcp

import (
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/google/gopacket/layers"
)

var dbSeq uint64

// newTestDB gives each test its own in-memory database, migrated and seeded
// with a single host in the given group.
func newTestDB(t *testing.T, group model.Group) *model.Host {
	t.Helper()

	// A distinct DSN per test keeps the shared cache from leaking rows between
	// them. The name must not come from t.Name(): subtest names contain "/"
	// and "#", and "#" is a URI fragment delimiter, which silently drops
	// mode=memory and opens a file on disk instead.
	dsn := fmt.Sprintf("file:test%d?mode=memory&cache=shared", atomic.AddUint64(&dbSeq, 1))
	if err := store.Open(dsn, false); err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := store.DB.AutoMigrate(&model.Host{}, &model.Option{}, &model.DeviceClass{}, &model.Group{}, &model.Image{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if res := store.DB.Create(&group); res.Error != nil {
		t.Fatalf("create group: %v", res.Error)
	}

	host := model.Host{
		HostForm: model.HostForm{
			IP:       "192.168.1.50",
			Mac:      "00:0c:29:00:00:01",
			Hostname: "esx01",
			GroupID:  model.NullInt32{},
		},
	}
	if err := host.GroupID.UnmarshalJSON([]byte(strconv.Itoa(group.ID))); err != nil {
		t.Fatalf("set group id: %v", err)
	}
	if res := store.DB.Create(&host); res.Error != nil {
		t.Fatalf("create host: %v", res.Error)
	}

	return &host
}

// request builds a DISCOVER asking for the given option codes.
func request(codes ...byte) *layers.DHCPv4 {
	mac, _ := net.ParseMAC("00:0c:29:00:00:01")
	return &layers.DHCPv4{
		Operation:    layers.DHCPOpRequest,
		HardwareType: layers.LinkTypeEthernet,
		Xid:          0x1234,
		ClientHWAddr: mac,
		Options: layers.DHCPOptions{
			layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeDiscover)}),
			layers.NewDHCPOption(layers.DHCPOptParamsRequest, codes),
		},
	}
}

func findOption(resp *layers.DHCPv4, code layers.DHCPOpt) (layers.DHCPOption, bool) {
	for _, o := range resp.Options {
		if o.Type == code {
			return o, true
		}
	}
	return layers.DHCPOption{}, false
}

func TestAddOptionsSubnetMask(t *testing.T) {
	host := newTestDB(t, model.Group{
		GroupForm: model.GroupForm{
			Name:       "test",
			Netmask:    "255.255.255.0",
			Gateway:    "192.168.1.1",
			BootMethod: "pxe",
		},
	})

	resp := &layers.DHCPv4{}
	if err := AddOptions(request(byte(layers.DHCPOptSubnetMask)), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	got, ok := findOption(resp, layers.DHCPOptSubnetMask)
	if !ok {
		t.Fatal("no subnet mask option in the response")
	}
	if want := []byte{255, 255, 255, 0}; string(got.Data) != string(want) {
		t.Errorf("subnet mask = %v, want %v", got.Data, want)
	}
}

// A group whose netmask is blank or malformed must not silently yield a
// 0.0.0.0 subnet mask, which the client would take to mean "everything is on
// link" and lose its default route.
func TestAddOptionsRejectsBadNetmask(t *testing.T) {
	for _, netmask := range []string{"", "not-a-mask", "99"} {
		t.Run(netmask, func(t *testing.T) {
			host := newTestDB(t, model.Group{
				GroupForm: model.GroupForm{
					Name:       "test",
					Netmask:    netmask,
					Gateway:    "192.168.1.1",
					BootMethod: "pxe",
				},
			})

			resp := &layers.DHCPv4{}
			err := AddOptions(request(byte(layers.DHCPOptSubnetMask)), resp, host, net.IPv4(192, 168, 1, 2))

			if got, ok := findOption(resp, layers.DHCPOptSubnetMask); ok && string(got.Data) == string([]byte{0, 0, 0, 0}) {
				t.Errorf("emitted a 0.0.0.0 subnet mask for netmask %q (err = %v)", netmask, err)
			}
		})
	}
}

// The classless static route option slices the gateway to the significant
// octets of the prefix. With a valid netmask and a blank gateway, To4 returns
// nil and the slice expression panics — inside the DHCP goroutine, which takes
// the whole daemon down.
func TestAddOptionsBlankGatewayDoesNotPanic(t *testing.T) {
	host := newTestDB(t, model.Group{
		GroupForm: model.GroupForm{
			Name:       "test",
			Netmask:    "255.255.255.0",
			Gateway:    "",
			BootMethod: "pxe",
		},
	})

	resp := &layers.DHCPv4{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AddOptions panicked on a blank gateway: %v", r)
		}
	}()

	_ = AddOptions(request(
		byte(layers.DHCPOptClasslessStaticRoute),
		byte(layers.DHCPOptRouter),
		byte(layers.DHCPOptSubnetMask),
	), resp, host, net.IPv4(192, 168, 1, 2))
}

func TestAddOptionsBootMethodToOption67(t *testing.T) {
	tests := []struct {
		bootMethod string
		want       string
	}{
		{"pxe", "mboot.efi"},
		{"https-dhcp", "https://192.168.1.2/esx/mboot.efi"},
		{"http-dhcp", "http://192.168.1.2/esx/mboot.efi"},
	}

	for _, tc := range tests {
		t.Run(tc.bootMethod, func(t *testing.T) {
			host := newTestDB(t, model.Group{
				GroupForm: model.GroupForm{
					Name:       "test",
					Netmask:    "255.255.255.0",
					Gateway:    "192.168.1.1",
					BootMethod: tc.bootMethod,
				},
			})

			resp := &layers.DHCPv4{}
			if err := AddOptions(request(67), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
				t.Fatalf("AddOptions: %v", err)
			}

			got, ok := findOption(resp, 67)
			if !ok {
				t.Fatal("no bootfile option in the response")
			}
			if string(got.Data) != tc.want {
				t.Errorf("bootfile = %q, want %q", got.Data, tc.want)
			}
		})
	}
}

// Option 67 is always requested by default, so an unset boot method must not
// produce an empty bootfile — the client would try to boot "" and time out.
func TestAddOptionsUnsetBootMethodEmitsNoBootfile(t *testing.T) {
	host := newTestDB(t, model.Group{
		GroupForm: model.GroupForm{
			Name:    "test",
			Netmask: "255.255.255.0",
			Gateway: "192.168.1.1",
		},
	})

	resp := &layers.DHCPv4{}
	if err := AddOptions(request(67), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	if got, ok := findOption(resp, 67); ok && len(got.Data) == 0 {
		t.Error("emitted an empty bootfile option for an unset boot method")
	}
}

func TestAddOptionsAlwaysIncludesLeaseAndServerID(t *testing.T) {
	host := newTestDB(t, model.Group{
		GroupForm: model.GroupForm{
			Name:       "test",
			Netmask:    "255.255.255.0",
			Gateway:    "192.168.1.1",
			BootMethod: "pxe",
		},
	})

	// Ask for nothing; the defaults must still be present.
	resp := &layers.DHCPv4{}
	if err := AddOptions(request(), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	for _, code := range []layers.DHCPOpt{
		layers.DHCPOptLeaseTime,
		layers.DHCPOptT1,
		layers.DHCPOptT2,
		layers.DHCPOptServerID,
	} {
		if _, ok := findOption(resp, code); !ok {
			t.Errorf("default option %d missing from the response", code)
		}
	}

	if got, ok := findOption(resp, layers.DHCPOptServerID); ok {
		if want := net.IPv4(192, 168, 1, 2).To4(); string(got.Data) != string(want) && len(got.Data) == 4 {
			t.Errorf("server id = %v, want %v", got.Data, want)
		}
	}
}

// The stored options have to actually reach the client. The query that loads
// them went missing when pools were removed, so the whole feature -- the
// Option model, its Level() precedence, the opcode encoding, the /options API
// -- was inert: an operator set an option, saw it stored, and it was never
// sent. Every test below fails against that state.
func TestAddOptionsSendsAGlobalOption(t *testing.T) {
	host := newTestDB(t, testGroup())

	// A global option: no host, no device class.
	createOption(t, model.Option{OptionForm: model.OptionForm{
		OpCode: byte(layers.DHCPOptDomainName),
		Data:   "example.com",
	}})

	resp := &layers.DHCPv4{}
	if err := AddOptions(request(byte(layers.DHCPOptDomainName)), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	got, ok := findOption(resp, layers.DHCPOptDomainName)
	if !ok {
		t.Fatal("the stored domain name never reached the response")
	}
	if string(got.Data) != "example.com" {
		t.Errorf("domain name = %q, want %q", got.Data, "example.com")
	}
}

// A host-specific option is more specific than a global one and must win.
func TestAddOptionsHostOptionBeatsGlobal(t *testing.T) {
	host := newTestDB(t, testGroup())

	createOption(t, model.Option{OptionForm: model.OptionForm{
		OpCode: byte(layers.DHCPOptDomainName),
		Data:   "global.example.com",
	}})
	createOption(t, model.Option{OptionForm: model.OptionForm{
		HostID: host.ID,
		OpCode: byte(layers.DHCPOptDomainName),
		Data:   "host.example.com",
	}})

	resp := &layers.DHCPv4{}
	if err := AddOptions(request(byte(layers.DHCPOptDomainName)), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	got, ok := findOption(resp, layers.DHCPOptDomainName)
	if !ok {
		t.Fatal("no domain name option in the response")
	}
	if string(got.Data) != "host.example.com" {
		t.Errorf("domain name = %q, want the host-specific value", got.Data)
	}
}

// One host's options must not be handed to another. This is the failure that
// would be hardest to notice: the client boots, just with someone else's
// settings.
func TestAddOptionsIgnoresAnotherHostsOptions(t *testing.T) {
	host := newTestDB(t, testGroup())

	other := model.Host{HostForm: model.HostForm{IP: "192.168.1.51", Mac: "00:0c:29:00:00:02"}}
	if res := store.DB.Create(&other); res.Error != nil {
		t.Fatalf("create other host: %v", res.Error)
	}

	createOption(t, model.Option{OptionForm: model.OptionForm{
		HostID: other.ID,
		OpCode: byte(layers.DHCPOptDomainName),
		Data:   "not-yours.example.com",
	}})

	resp := &layers.DHCPv4{}
	if err := AddOptions(request(byte(layers.DHCPOptDomainName)), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	if got, ok := findOption(resp, layers.DHCPOptDomainName); ok {
		t.Errorf("got another host's option: %q", got.Data)
	}
}

// An option the client did not ask for is still sent, at the end. Clients
// commonly omit codes from the parameter request list and use them anyway.
func TestAddOptionsSendsUnrequestedOptions(t *testing.T) {
	host := newTestDB(t, testGroup())

	createOption(t, model.Option{OptionForm: model.OptionForm{
		OpCode: byte(layers.DHCPOptDomainName),
		Data:   "example.com",
	}})

	resp := &layers.DHCPv4{}
	// Ask only for the subnet mask.
	if err := AddOptions(request(byte(layers.DHCPOptSubnetMask)), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	if _, ok := findOption(resp, layers.DHCPOptDomainName); !ok {
		t.Error("an option that was stored but not requested should still be sent")
	}
}

// An option that cannot be encoded must not take the rest of the response
// with it: the client still needs its boot file and its subnet mask.
func TestAddOptionsSkipsAnUnencodableOption(t *testing.T) {
	host := newTestDB(t, testGroup())

	createOption(t, model.Option{OptionForm: model.OptionForm{
		OpCode: byte(layers.DHCPOptRouter),
		Data:   "not-an-ip-address",
	}})

	resp := &layers.DHCPv4{}
	if err := AddOptions(request(byte(layers.DHCPOptRouter), byte(layers.DHCPOptSubnetMask)), resp, host, net.IPv4(192, 168, 1, 2)); err != nil {
		t.Fatalf("AddOptions: %v", err)
	}

	if _, ok := findOption(resp, layers.DHCPOptSubnetMask); !ok {
		t.Error("a bad option stopped the good ones being sent")
	}
}

func createOption(t *testing.T, o model.Option) {
	t.Helper()

	if res := store.DB.Create(&o); res.Error != nil {
		t.Fatalf("create option: %v", res.Error)
	}
}
