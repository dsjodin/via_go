package dhcpd

import (
	"net"
	"testing"

	"github.com/google/gopacket/layers"
	"github.com/maxiepax/go-via/db"
	"github.com/maxiepax/go-via/models"
)

var serverIP = net.IPv4(192, 168, 1, 2)

func testGroup() models.Group {
	return models.Group{
		GroupForm: models.GroupForm{
			Name:       "test",
			Netmask:    "255.255.255.0",
			Gateway:    "192.168.1.1",
			BootMethod: "pxe",
		},
	}
}

// packet builds a DHCP message of the given type from the seeded host's MAC.
func packet(msgType layers.DHCPMsgType, opts ...layers.DHCPOption) *layers.DHCPv4 {
	mac, _ := net.ParseMAC("00:0c:29:00:00:01")
	p := &layers.DHCPv4{
		Operation:    layers.DHCPOpRequest,
		HardwareType: layers.LinkTypeEthernet,
		Xid:          0x1234,
		ClientHWAddr: mac,
		Options: layers.DHCPOptions{
			layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(msgType)}),
		},
	}
	p.Options = append(p.Options, opts...)
	return p
}

func msgType(t *testing.T, resp *layers.DHCPv4) layers.DHCPMsgType {
	t.Helper()
	got, ok := findOption(resp, layers.DHCPOptMessageType)
	if !ok || len(got.Data) == 0 {
		t.Fatal("response carries no message type option")
	}
	return layers.DHCPMsgType(got.Data[0])
}

func TestProcessDiscoverOffersTheReservedAddress(t *testing.T) {
	host := newTestDB(t, testGroup())
	db.DB.Model(host).Update("reimage", true)

	resp, err := processDiscover(packet(layers.DHCPMsgTypeDiscover), serverIP, serverIP)
	if err != nil {
		t.Fatalf("processDiscover: %v", err)
	}

	if got := msgType(t, resp); got != layers.DHCPMsgTypeOffer {
		t.Errorf("message type = %v, want Offer", got)
	}
	if !resp.YourClientIP.Equal(net.ParseIP("192.168.1.50")) {
		t.Errorf("offered %v, want 192.168.1.50", resp.YourClientIP)
	}
	if !resp.NextServerIP.Equal(serverIP.To4()) {
		t.Errorf("next server = %v, want %v", resp.NextServerIP, serverIP)
	}
}

func TestProcessDiscoverIgnoresUnknownMAC(t *testing.T) {
	newTestDB(t, testGroup())

	req := packet(layers.DHCPMsgTypeDiscover)
	req.ClientHWAddr, _ = net.ParseMAC("00:0c:29:ff:ff:ff")

	resp, err := processDiscover(req, serverIP, serverIP)
	if err == nil {
		t.Fatal("expected an error for an unknown mac address")
	}
	if resp != nil {
		t.Errorf("expected no response for an unknown mac, got %v", resp)
	}
}

// A host that is not flagged for re-imaging must not be answered at all —
// answering would hand a lease to a production machine that already has one.
func TestProcessDiscoverIgnoresHostNotFlaggedForReimage(t *testing.T) {
	host := newTestDB(t, testGroup())
	db.DB.Model(host).Update("reimage", false)

	if _, err := processDiscover(packet(layers.DHCPMsgTypeDiscover), serverIP, serverIP); err == nil {
		t.Fatal("expected an error for a host that is not flagged for re-imaging")
	}
}

func TestProcessRequestAcksTheReservedAddress(t *testing.T) {
	host := newTestDB(t, testGroup())
	db.DB.Model(host).Update("reimage", true)

	req := packet(layers.DHCPMsgTypeRequest,
		layers.NewDHCPOption(layers.DHCPOptRequestIP, net.ParseIP("192.168.1.50").To4()))

	resp, err := processRequest(req, serverIP, serverIP)
	if err != nil {
		t.Fatalf("processRequest: %v", err)
	}

	if got := msgType(t, resp); got != layers.DHCPMsgTypeAck {
		t.Errorf("message type = %v, want Ack", got)
	}
	if !resp.YourClientIP.Equal(net.ParseIP("192.168.1.50")) {
		t.Errorf("acked %v, want 192.168.1.50", resp.YourClientIP)
	}
}

// The reservation is the whole point of the host record. A client asking for
// anything else — a stale lease from another network, or a client simply
// claiming an address — must be refused, not confirmed.
func TestProcessRequestNaksAnAddressThatIsNotTheReservation(t *testing.T) {
	host := newTestDB(t, testGroup())
	db.DB.Model(host).Update("reimage", true)

	req := packet(layers.DHCPMsgTypeRequest,
		layers.NewDHCPOption(layers.DHCPOptRequestIP, net.ParseIP("192.168.1.99").To4()))

	resp, err := processRequest(req, serverIP, serverIP)
	if err != nil {
		// Refusing by error rather than by NAK is acceptable; silently
		// confirming the wrong address is not.
		return
	}

	if got := msgType(t, resp); got != layers.DHCPMsgTypeNak {
		t.Errorf("message type = %v, want Nak; server confirmed %v for a host reserved 192.168.1.50",
			got, resp.YourClientIP)
	}
}

func TestProcessRequestIgnoresUnknownMAC(t *testing.T) {
	newTestDB(t, testGroup())

	req := packet(layers.DHCPMsgTypeRequest,
		layers.NewDHCPOption(layers.DHCPOptRequestIP, net.ParseIP("192.168.1.50").To4()))
	req.ClientHWAddr, _ = net.ParseMAC("00:0c:29:ff:ff:ff")

	if _, err := processRequest(req, serverIP, serverIP); err == nil {
		t.Fatal("expected an error for an unknown mac address")
	}
}

func TestProcessPacketDispatch(t *testing.T) {
	host := newTestDB(t, testGroup())
	db.DB.Model(host).Update("reimage", true)

	tests := []struct {
		msgType     layers.DHCPMsgType
		wantReply   bool
		wantMsgType layers.DHCPMsgType
	}{
		{layers.DHCPMsgTypeDiscover, true, layers.DHCPMsgTypeOffer},
		{layers.DHCPMsgTypeRequest, true, layers.DHCPMsgTypeAck},
		{layers.DHCPMsgTypeInform, false, 0},
		{layers.DHCPMsgTypeDecline, false, 0},
		{layers.DHCPMsgTypeUnspecified, false, 0},
		{layers.DHCPMsgTypeOffer, false, 0},
		{layers.DHCPMsgTypeAck, false, 0},
		{layers.DHCPMsgTypeNak, false, 0},
	}

	for _, tc := range tests {
		req := packet(tc.msgType,
			layers.NewDHCPOption(layers.DHCPOptRequestIP, net.ParseIP("192.168.1.50").To4()))

		resp, err := processPacket(tc.msgType, req, serverIP, serverIP)

		if !tc.wantReply {
			if err == nil {
				t.Errorf("%v: expected an error, got response %v", tc.msgType, resp)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v: unexpected error: %v", tc.msgType, err)
			continue
		}
		if got := msgType(t, resp); got != tc.wantMsgType {
			t.Errorf("%v: replied %v, want %v", tc.msgType, got, tc.wantMsgType)
		}
	}
}

func TestFindMsgType(t *testing.T) {
	for _, want := range []layers.DHCPMsgType{
		layers.DHCPMsgTypeDiscover,
		layers.DHCPMsgTypeRequest,
		layers.DHCPMsgTypeAck,
		layers.DHCPMsgTypeNak,
	} {
		if got := findMsgType(packet(want)); got != want {
			t.Errorf("findMsgType = %v, want %v", got, want)
		}
	}

	// A packet with no message type option must not be mistaken for a valid
	// one; the zero value is Unspecified.
	bare := &layers.DHCPv4{}
	if got := findMsgType(bare); got != layers.DHCPMsgTypeUnspecified {
		t.Errorf("findMsgType on a bare packet = %v, want Unspecified", got)
	}
}

func TestBuildHeadersBroadcastsWhenClientHasNoAddress(t *testing.T) {
	mac, _ := net.ParseMAC("00:0c:29:00:00:02")
	srcEth := &layers.Ethernet{SrcMAC: mac}

	// A client mid-DISCOVER has no address yet, so the reply cannot be
	// unicast — it has to go to the broadcast address on the client port.
	headers := buildHeaders(mac, serverIP, srcEth, &layers.IPv4{SrcIP: net.IPv4zero}, &layers.UDP{})
	if len(headers) != 3 {
		t.Fatalf("buildHeaders returned %d layers, want 3", len(headers))
	}

	ip4, ok := headers[1].(*layers.IPv4)
	if !ok {
		t.Fatal("second layer is not IPv4")
	}
	if !ip4.DstIP.Equal(net.IPv4bcast) {
		t.Errorf("destination = %v, want 255.255.255.255", ip4.DstIP)
	}

	udp, ok := headers[2].(*layers.UDP)
	if !ok {
		t.Fatal("third layer is not UDP")
	}
	if udp.DstPort != 68 {
		t.Errorf("destination port = %v, want 68", udp.DstPort)
	}
}

func TestBuildHeadersUnicastsToAClientWithAnAddress(t *testing.T) {
	mac, _ := net.ParseMAC("00:0c:29:00:00:02")
	srcEth := &layers.Ethernet{SrcMAC: mac}
	client := net.IPv4(192, 168, 1, 50)

	headers := buildHeaders(mac, serverIP, srcEth, &layers.IPv4{SrcIP: client}, &layers.UDP{})

	ip4 := headers[1].(*layers.IPv4)
	if !ip4.DstIP.Equal(client) {
		t.Errorf("destination = %v, want %v", ip4.DstIP, client)
	}

	udp := headers[2].(*layers.UDP)
	if udp.DstPort != 67 {
		t.Errorf("destination port = %v, want 67", udp.DstPort)
	}
}
