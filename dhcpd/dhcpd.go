package dhcpd

import (
	"bytes"
	//"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	//"time"

	"github.com/google/gopacket/layers"
	//"github.com/maxiepax/go-via/api"
	"github.com/maxiepax/go-via/db"
	"github.com/maxiepax/go-via/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/gopacket"
	"github.com/mdlayher/raw"
)

func processPacket(t layers.DHCPMsgType, req *layers.DHCPv4, sourceNet net.IP, ip net.IP) (resp *layers.DHCPv4, err error) {
	switch t {
	case layers.DHCPMsgTypeDiscover:
		return processDiscover(req, sourceNet, ip)
	case layers.DHCPMsgTypeRequest:
		return processRequest(req, sourceNet, ip)
	case layers.DHCPMsgTypeRelease:
		//return "Release"
	case layers.DHCPMsgTypeInform:
		return nil, fmt.Errorf("ignored, inform type")
	case layers.DHCPMsgTypeDecline:
		return nil, fmt.Errorf("an ip address conflict was detected, this should not be possible")
		//return processDecline(req, sourceNet, ip)
	case layers.DHCPMsgTypeUnspecified:
		return nil, fmt.Errorf("ignored, unspecified type")
	case layers.DHCPMsgTypeOffer:
		return nil, fmt.Errorf("ignored, offer type")
	case layers.DHCPMsgTypeAck:
		return nil, fmt.Errorf("ignored, ack type")
	case layers.DHCPMsgTypeNak:
		return nil, fmt.Errorf("ignored, nak type")
	}
	return nil, fmt.Errorf("unknown dhcp request type")
}

func processDiscover(req *layers.DHCPv4, sourceNet net.IP, ip net.IP) (resp *layers.DHCPv4, err error) {
	// Find all reimage hosts that is not yet assigned a pool
	/*
	var reimageHosts []models.Host
	if res := db.DB.Where("pool_id IS NULL").Where("reimage = 1").Find(&reimageHosts); res.Error != nil {
		if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, res.Error
		}
	}
	*/

	/*
	pool, err := api.FindPool(sourceNet.String())
	if err != nil {
		return nil, err
	}
	*/

	/*

	// Make a list of all reimage and pool hosts
	hosts := append(reimageHosts, pool.Hosts...)

	// Search in the list for our mac address
	var leaseIP net.IP
	var lease *models.Host
	for _, v := range hosts {
		// Make sure the reimage IP is within the pool
		parsedIp := net.ParseIP(v.IP)
		ok, _ := pool.Contains(parsedIp)

		// Check so we havent given someone else this IP
		err := pool.IsAvailableExcept(parsedIp, req.ClientHWAddr.String())

		if v.Mac == req.ClientHWAddr.String() && ok && err == nil {
			leaseIP = parsedIp
			lease = &v
			break
		}
	}
	*/


	/*
	// Dont answer pools with "only serve requested" flag set
	if pool.OnlyServeReimage && (lease == nil || !lease.Reimage) {
		return nil, fmt.Errorf("ignored because mac address is not flagged for re-imaging")
	}
	*/

	host, _ := findHostByMAC(req.ClientHWAddr.String())

	resp = &layers.DHCPv4{
		Operation:    layers.DHCPOpReply,
		HardwareType: layers.LinkTypeEthernet,
		Xid:          req.Xid,
		//YourClientIP: leaseIP,
		YourClientIP: net.ParseIP(host.IP),
		RelayAgentIP: req.RelayAgentIP,
		ClientHWAddr: req.ClientHWAddr,
		NextServerIP: ip.To4(),
	}

	resp.Options = append(resp.Options, layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeOffer)}))

	AddOptions(req, resp, host, ip)

	//req *layers.DHCPv4, resp *layers.DHCPv4, pool models.PoolWithHosts, lease *models.Host, ip net.IP

	return resp, nil
}

func processRequest(req *layers.DHCPv4, sourceNet net.IP, ip net.IP) (*layers.DHCPv4, error) {

	// Find all reimage hosts that is not yet assigned a pool
	/*
	var reimageHosts []models.Host
	if res := db.DB.Where("pool_id IS NULL").Where("reimage = 1").Find(&reimageHosts); res.Error != nil {
		if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, res.Error
		}
	}

	// Figure out and get the pool
	pool, err := api.FindPool(sourceNet.String())
	if err != nil {
		return nil, err
	}

	// Make a list of all reimage and pool hosts
	hosts := append(reimageHosts, pool.Hosts...)
	*/


	// Extract the requested IP
	var requestedIP net.IP = req.ClientIP
	for _, v := range req.Options {
		if v.Type == layers.DHCPOptRequestIP {
			requestedIP = net.IP(v.Data)
		}
	}


	// Start building the response
	resp := &layers.DHCPv4{
		Operation:    layers.DHCPOpReply,
		HardwareType: layers.LinkTypeEthernet,
		Xid:          req.Xid,
		RelayAgentIP: req.RelayAgentIP,
		ClientHWAddr: req.ClientHWAddr,
		NextServerIP: ip.To4(),
	}

	host, _ := findHostByMAC(req.ClientHWAddr.String())

	/*
	// Try to find the lease in our host list
	var lease *models.Host
	for _, v := range hosts {
		// Check so the IP is part of the pool
		parsedIp := net.ParseIP(v.IP)
		ok, _ := pool.Contains(parsedIp)

		// Check so we havent given someone else this IP
		err := pool.IsAvailableExcept(parsedIp, req.ClientHWAddr.String())

		if v.Mac == req.ClientHWAddr.String() && v.IP != requestedIP.String() && v.Expires.After(time.Now()) && ok && err == nil {
			logrus.WithFields(logrus.Fields{
				"pool":      pool.ID,
				"expected":  v.IP,
				"requested": requestedIP.String(),
			}).Warn("dhcp: wrong ip requested")
			resp.Options = append(resp.Options, layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeNak)}))
			return resp, nil
		}

		if v.Mac == req.ClientHWAddr.String() {
			foundLease := models.Host(v)
			lease = &foundLease
		}
	}

	// Check if the requested IP is available
	if lease == nil || lease.IP != requestedIP.String() {
		if err := pool.IsAvailable(requestedIP); err != nil {
			logrus.WithFields(logrus.Fields{
				"pool":      pool.ID,
				"requested": requestedIP.String(),
				"err":       err,
			}).Warnf("dhcp: the requested ip is not available")
			resp.Options = append(resp.Options, layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeNak)}))
			return resp, nil
		}
	}

	// Make sure the address isnt already used
	if lease != nil {
		if err := pool.IsAvailableExcept(requestedIP, req.ClientHWAddr.String()); err != nil {
			logrus.WithFields(logrus.Fields{
				"pool":      pool.ID,
				"requested": requestedIP.String(),
				"err":       err,
			}).Warnf("dhcp: the requested ip is not available (used by someone else)")
			resp.Options = append(resp.Options, layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeNak)}))
			return resp, nil
		}
	}

	// Dont answer pools with "only serve requested" flag set
	if pool.OnlyServeReimage && (lease == nil || !lease.Reimage) {
		return nil, fmt.Errorf("ignored because mac address is not flagged for reimaging")
	}

	// Its a new lease!
	if lease == nil {
		lease = &models.Host{
			HostForm: models.HostForm{
				Mac:      req.ClientHWAddr.String(),
				Hostname: "-",
				Reimage:  false,
			},
		}
	}
	*/

	/*
	// Respond with the same hostname
	for _, v := range req.Options {
		if v.Type == layers.DHCPOptHostname {
			lease.Hostname = string(v.Data)
		}
	}
	*/
	resp.YourClientIP = requestedIP
	/*
	pool, err := api.FindPool("172.16.60.0")
	if err != nil {
		return nil, err
	}
	*/

	resp.Options = append(resp.Options, layers.NewDHCPOption(layers.DHCPOptMessageType, []byte{byte(layers.DHCPMsgTypeAck)}))
	AddOptions(req, resp, host, ip)


	/*
	lease.IP = requestedIP.String()
	lease.PoolID = models.NullInt32{sql.NullInt32{int32(pool.ID), true}}
	lease.LastSeenRelay = req.RelayAgentIP.String()
	if (lease.FirstSeen == time.Time{}) {
		lease.FirstSeen = time.Now()
	}
	lease.LastSeen = time.Now()
	lease.Expires = time.Now().Add(3600 * time.Second)
	lease.MissingOptions = listMissingOptions(req, resp)

	if lease.ID == 0 {
		db.DB.Create(lease)
	} else {
		// Remove the previous record if there is any
		db.DB.Exec("DELETE FROM hosts WHERE ip=? AND reimage=0 AND expires <= datetime('now', 'utc')", lease.IP)
		db.DB.Save(lease)
	}
	*/

	return resp, nil
}

func listMissingOptions(req *layers.DHCPv4, resp *layers.DHCPv4) string {
	requested := map[byte]struct{}{}
	for _, v := range req.Options {
		if v.Type == layers.DHCPOptParamsRequest {
			for _, v := range v.Data {
				requested[v] = struct{}{}
			}
		}
	}

	for _, v := range resp.Options {
		if _, ok := requested[byte(v.Type)]; ok {
			delete(requested, byte(v.Type))
		}
	}

	var list []string
	for k := range requested {
		list = append(list, strconv.Itoa(int(k)))
	}

	return strings.Join(list, ",")
}

// a IP address conflict was detected, add/update the address table to block that address from being used for a while (lease time)
/*
func processDecline(req *layers.DHCPv4, sourceNet net.IP, ip net.IP) (*layers.DHCPv4, error) {

	pool, err := api.FindPool(sourceNet.String())
	if err != nil {
		return nil, err
	}

	var requestedIP net.IP
	for _, v := range req.Options {
		if v.Type == layers.DHCPOptRequestIP {
			requestedIP = net.IP(v.Data)
		}
	}

	// Try to find the lease in our host history
	var lease *models.Host
	for _, v := range pool.Hosts {
		if v.IP == requestedIP.To4().String() {
			lease = &v
		}
	}

	// Its an unknown device
	if lease == nil {
		lease = &models.Host{
			HostForm: models.HostForm{
				IP:       requestedIP.String(),
				Hostname: "-",
				Reimage:  false,
			},
		}
	}

	lease.Mac = ""
	lease.PoolID = models.NullInt32{sql.NullInt32{int32(pool.ID), true}}
	lease.LastSeenRelay = req.RelayAgentIP.String()
	lease.LastSeen = time.Now()
	lease.Expires = time.Now().Add(3600 * time.Second)

	if lease.ID == 0 {
		db.DB.Create(lease)
	} else {
		db.DB.Save(lease)
	}

	return nil, nil
}
*/

// AddOptions will try to add all requested options and the manually specified ones to the response
/* func AddOptions(req *layers.DHCPv4, resp *layers.DHCPv4, pool models.PoolWithHosts, lease *models.Host, ip net.IP) error { */
func AddOptions(req *layers.DHCPv4, resp *layers.DHCPv4, lease *models.Host, ip net.IP) error {

	var options []models.Option
	/*
	var leaseID interface{}

	if lease != nil {
		leaseID = lease.ID
	}
	*/

	// Try to find the device class
	var deviceClass models.DeviceClass
	for _, v := range req.Options {
		if v.Type == 60 { // Vendor class
			db.DB.Where("? LIKE '%' || vendor_class || '%'", string(v.Data)).First(&deviceClass)
		}
	}

	/*
	if res := db.DB.Where("((pool_id = 0 AND device_class_id = 0 AND host_id = 0) OR pool_id = ? OR host_id = ?) AND (device_class_id = 0 OR device_class_id = ?)", pool.ID, leaseID, deviceClass.ID).Order("device_class_id desc").Order("host_id desc").Order("pool_id desc").Find(&options); res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {

		return res.Error
	}
	*/

	// Group options by opcode
	byOpCode := make(map[byte][]models.Option)
	for _, v := range options {
		if byOpCode[v.OpCode] == nil {
			byOpCode[v.OpCode] = make([]models.Option, 0)
		}

		// Only add the highest level options to the list
		// The level is decided on pool_id and host_id fields
		// addess+device_class specific = 5
		// pool+device_class specific = 4
		// global+device_class = 3
		// addess specific = 2
		// pool specific = 1
		// global = 0
		if len(byOpCode[v.OpCode]) == 0 || v.Level() >= byOpCode[v.OpCode][0].Level() {
			byOpCode[v.OpCode] = append(byOpCode[v.OpCode], v)
		}
	}

	// Extract the order of the requested options
	requestedOptions := map[byte]struct{}{}
	for _, v := range req.Options {
		if v.Type == layers.DHCPOptParamsRequest {
			for _, v := range v.Data {
				requestedOptions[v] = struct{}{}
			}
		}
	}

	defaultOptions := []byte{
		byte(layers.DHCPOptT1),
		byte(layers.DHCPOptT2),
		byte(layers.DHCPOptLeaseTime),
		byte(layers.DHCPOptServerID),
		byte(67),
	}
	for _, v := range defaultOptions {
		if _, ok := requestedOptions[v]; !ok {
			requestedOptions[v] = struct{}{}
		}
	}

	//get the hosts group object to determine the boot method used.
	var h models.Host
	if err := db.DB.Preload("Group").First(&h, lease.ID).Error; err == nil {
	}
	bootmethod := h.Group.BootMethod

	// Add the requested options to the response
	var leaseTime float64 = float64(3600)
	/*
	if leaseTime == 0 {
		leaseTime = 3600
	}
	*/
	for opCode := range requestedOptions {
		if options, ok := byOpCode[opCode]; ok {
			for _, v := range options {
				dhcpOpt, _, err := v.ToDHCPOption() // TODO: fix merge
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"opcode": opCode,
						"name":   layers.DHCPOpt(opCode).String(),
						"err":    err,
					}).Error("dhcp: failed to encode dhcp option")
					continue
				}

				resp.Options = append(resp.Options, dhcpOpt)
			}
			delete(byOpCode, opCode)
			continue
		}

		// Try to generate the missing option
		code := layers.DHCPOpt(opCode)
		switch code {
		case 67:
			if bootmethod == "pxe" {
				resp.Options = append(resp.Options, layers.NewDHCPOption(code, []byte("mboot.efi")))
				logrus.WithFields(logrus.Fields{
					"boot method": bootmethod,
					"filepath":    "mboot.efi",
				}).Info("dhcp")
			} else if bootmethod == "https-dhcp" {
				resp.Options = append(resp.Options, layers.NewDHCPOption(code, []byte("https://"+ip.String()+"/esx/mboot.efi")))
				logrus.WithFields(logrus.Fields{
					"boot method": bootmethod,
					"filepath":    "https://"+ip.String()+"/esx/mboot.efi",
				}).Info("dhcp")
			} else if bootmethod == "http-dhcp" {
				resp.Options = append(resp.Options, layers.NewDHCPOption(code, []byte("http://"+ip.String()+"/esx/mboot.efi")))
				logrus.WithFields(logrus.Fields{
					"boot method": bootmethod,
					"filepath":    "http://"+ip.String()+"/esx/mboot.efi",
				}).Info("dhcp")
			}
		case layers.DHCPOptSubnetMask:
			cidrMask, _ := NetmaskToCIDR(h.Group.Netmask)
			resp.Options = append(resp.Options, layers.NewDHCPOption(code, net.CIDRMask(cidrMask, 32)))
		case layers.DHCPOptClasslessStaticRoute:
			cidrMask, _ := NetmaskToCIDR(h.Group.Netmask)
			var b bytes.Buffer
			b.Write([]byte{byte(cidrMask)})

			// Only write the non-zero octets.
			dstLen := (cidrMask + 7) / 8
			b.Write(net.ParseIP(h.Group.Gateway).To4()[:dstLen])

			b.Write(net.ParseIP(h.Group.Gateway).To4())
			resp.Options = append(resp.Options, layers.NewDHCPOption(code, b.Bytes()))
		case layers.DHCPOptRouter:
			resp.Options = append(resp.Options, layers.NewDHCPOption(code, net.ParseIP(h.Group.Gateway).To4()))
		case layers.DHCPOptBroadcastAddr:
			/*
			cidrMask, _ := NetmaskToCIDR(h.Group.Netmask)
			b, err := BroadcastAddress(h.Group.Gateway, strconv.Itoa(cidrMask))
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"opcode": opCode,
					"name":   layers.DHCPOpt(opCode).String(),
					"err":    err,
				}).Warn("dhcp: could not get broadcast address")
				continue
			}
			resp.Options = append(resp.Options, layers.NewDHCPOption(code, b))
			*/
			cidrMask, _ := NetmaskToCIDR(h.Group.Netmask)
			bcastStr, err := BroadcastAddress(h.Group.Gateway, strconv.Itoa(cidrMask))
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"opcode": opCode,
					"name":   layers.DHCPOpt(opCode).String(),
					"err":    err,
				}).Warn("dhcp: could not get broadcast address")
				continue
			}
			bcastIP := net.ParseIP(bcastStr).To4()
			if bcastIP == nil {
				logrus.WithFields(logrus.Fields{
					"broadcast": bcastStr,
				}).Warn("dhcp: invalid broadcast IP")
				continue
			}
    		resp.Options = append(resp.Options, layers.NewDHCPOption(code, bcastIP))
		case layers.DHCPOptT1:
			resp.Options = append(resp.Options, models.NewUint32Option(layers.DHCPOptT1, int(leaseTime*0.5))) // renewal time
		case layers.DHCPOptT2:
			resp.Options = append(resp.Options, models.NewUint32Option(layers.DHCPOptT2, int(leaseTime*0.875))) // rebind time
		case layers.DHCPOptLeaseTime:
			resp.Options = append(resp.Options, models.NewUint32Option(layers.DHCPOptLeaseTime, int(leaseTime))) // lease time
		case layers.DHCPOptServerID:
			resp.Options = append(resp.Options, layers.NewDHCPOption(code, ip))
		default:
			// Everything failed :/
			logrus.WithFields(logrus.Fields{
				"opcode": opCode,
				"name":   layers.DHCPOpt(opCode).String(),
			}).Debug("dhcp: could not find the requested option", opCode, layers.DHCPOpt(opCode).String())
		}

	}
	spew.Dump(resp.Options)

	// Add the remaining options (that werent requested) in the end
	for opCode, options := range byOpCode {
		for _, v := range options {
			dhcpOpt, _, err := v.ToDHCPOption() // TODO: fix merge
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"opcode": opCode,
					"name":   layers.DHCPOpt(opCode).String(),
					"err":    err,
				}).Error("dhcp: failed to encode dhcp option")
				continue
			}

			resp.Options = append(resp.Options, dhcpOpt)
		}
	}

	return nil
}

func Init(intf string) {
	//create the device classes for x86 and arm
	//64bit x86 UEFI PXE
	var x86_64_pxe models.DeviceClass
	if res := db.DB.FirstOrCreate(&x86_64_pxe, models.DeviceClass{DeviceClassForm: models.DeviceClassForm{Name: "PXE-UEFI_x64", VendorClass: "PXEClient:Arch:00007"}}); res.Error != nil {
		logrus.Warning(res.Error)
	}

	//64bit x86 UEFI HTTP
	var x86_64_http models.DeviceClass
	if res := db.DB.FirstOrCreate(&x86_64_http, models.DeviceClass{DeviceClassForm: models.DeviceClassForm{Name: "HTTP-UEFI_x64", VendorClass: "HTTPClient:Arch:00016"}}); res.Error != nil {
		logrus.Warning(res.Error)
	}

	//64bit ARM UEFI PXE
	var arm_64_pxe models.DeviceClass
	if res := db.DB.FirstOrCreate(&arm_64_pxe, models.DeviceClass{DeviceClassForm: models.DeviceClassForm{Name: "PXE-UEFI_ARM64", VendorClass: "PXEClient:Arch:00011"}}); res.Error != nil {
		logrus.Warning(res.Error)
	}

	//64bit ARM UEFI HTTP
	var arm_64_http models.DeviceClass
	if res := db.DB.FirstOrCreate(&arm_64_http, models.DeviceClass{DeviceClassForm: models.DeviceClassForm{Name: "HTTP-UEFI_ARM64", VendorClass: "HTTPClient:Arch:00011"}}); res.Error != nil {
		logrus.Warning(res.Error)
	}

	// Select interface to used
	ifi, err := net.InterfaceByName(intf)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"if":  intf,
			"err": err,
		}).Fatalf("dhcp: failed to open interface")
	}

	// Find the ip-address
	ip, ipNet, err := FindIPv4Addr(ifi)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"if":  intf,
			"err": err,
		}).Fatalf("dhcp: failed to get interface IPv4 address")
	}

	mac := ifi.HardwareAddr

	// Open a raw socket using ethertype 0x0800 (IPv4)
	c, err := raw.ListenPacket(ifi, 0x0800, &raw.Config{})
	if err != nil {
		logrus.Fatalf("dhcp: failed to listen: %v", err)
	}
	defer c.Close()

	logrus.WithFields(logrus.Fields{
		"mac": mac,
		"ip":  ip,
		"int": intf,
	}).Infof("Starting dhcp server")

	// Accept frames up to interface's MTU in size
	b := make([]byte, ifi.MTU)

	// Keep reading frames
	for {
		n, src, err := c.ReadFrom(b)
		if err != nil {
			logrus.Fatalf("dhcp: failed to receive message: %v", err)
		}

		packet := gopacket.NewPacket(b[:n], layers.LayerTypeEthernet, gopacket.Default)

		ethLayer := packet.Layer(layers.LayerTypeEthernet)
		ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		dhcpLayer := packet.Layer(layers.LayerTypeDHCPv4)

		if ethLayer != nil && ipv4Layer != nil && udpLayer != nil && dhcpLayer != nil {
			eth, _ := ethLayer.(*layers.Ethernet)
			ipv4, _ := ipv4Layer.(*layers.IPv4)
			udp, _ := udpLayer.(*layers.UDP)
			req, _ := dhcpLayer.(*layers.DHCPv4)

			t := findMsgType(req)
			sourceNet := ip
			source := "broadcast"
			if ipNet != nil && !ipNet.Contains(ipv4.SrcIP) && !ipv4.SrcIP.Equal(net.IPv4zero) {
				sourceNet = ipv4.SrcIP
				source = "unicast"
			}

			if (req.RelayAgentIP != nil && !req.RelayAgentIP.Equal(net.IP{0, 0, 0, 0})) {
				sourceNet = req.RelayAgentIP
				source = "relayed"
			}

			resp, err := processPacket(t, req, sourceNet, ip)

			if err != nil {
				logrus.WithFields(logrus.Fields{
					"type":       t.String(),
					"client-mac": req.ClientHWAddr.String(),
					"source":     sourceNet.String(),
					"relay":      req.RelayAgentIP,
					"error":      err,
				}).Warnf("dhcp: failed to process %s %s", source, t)
				continue
			}

			// Copy some information from the request like option 82 (agent info) to the response
			resp.Flags = req.Flags
			for _, v := range req.Options {
				if v.Type == layers.DHCPOptClientID {
					resp.Options = append(resp.Options, v)
				}
				if v.Type == layers.DHCPOptHostname {
					resp.Options = append(resp.Options, v)
				}
				if v.Type == 82 {
					resp.Options = append(resp.Options, v)
				}
			}

			layers := buildHeaders(mac, ip, eth, ipv4, udp)
			layers = append(layers, resp)

			buf := gopacket.NewSerializeBuffer()
			opts := gopacket.SerializeOptions{
				FixLengths:       true,
				ComputeChecksums: true,
			}
			err = gopacket.SerializeLayers(buf, opts, layers...)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"response":   findMsgType(resp).String(),
					"client-mac": req.ClientHWAddr.String(),
					"ip":         resp.YourClientIP,
					"relay":      req.RelayAgentIP,
				}).Warnf("dhcp: failed to serialise response to %s %s", source, t)
				continue
			}

			c.WriteTo(buf.Bytes(), src)

			logrus.WithFields(logrus.Fields{
				"response":   findMsgType(resp).String(),
				"client-mac": req.ClientHWAddr.String(),
				"ip":         resp.YourClientIP,
				"relay":      req.RelayAgentIP,
			}).Infof("dhcp: answered %s %s with %s", source, t, findMsgType(resp))
			for _, v := range resp.Options {
				logrus.Debug(v)
			}
		}
	}
}

func findMsgType(p *layers.DHCPv4) layers.DHCPMsgType {
	var msgType layers.DHCPMsgType
	for _, o := range p.Options {
		if o.Type == layers.DHCPOptMessageType {
			msgType = layers.DHCPMsgType(o.Data[0])
		}
	}

	return msgType
}

func buildHeaders(mac net.HardwareAddr, ip net.IP, srcEth *layers.Ethernet, srcIP4 *layers.IPv4, srcUDP *layers.UDP) []gopacket.SerializableLayer {
	eth := &layers.Ethernet{
		SrcMAC:       mac,
		DstMAC:       srcEth.SrcMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip4 := &layers.IPv4{
		SrcIP:    ip,
		DstIP:    srcIP4.SrcIP,
		Version:  4,
		TOS:      0x10,
		TTL:      128,
		Protocol: layers.IPProtocolUDP,
		Flags:    layers.IPv4DontFragment,
	}

	udp := &layers.UDP{
		SrcPort: 67, // bootps
		DstPort: 67, // bootps
	}

	// Answer to broadcast address if source address is 0.0.0.0
	if srcIP4.SrcIP.Equal(net.IPv4zero) {
		ip4.DstIP = net.IPv4(255, 255, 255, 255)
		udp.DstPort = 68
	}

	udp.SetNetworkLayerForChecksum(ip4)

	return []gopacket.SerializableLayer{eth, ip4, udp}
}

func FindIPv4Addr(ifi *net.Interface) (net.IP, *net.IPNet, error) {
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, nil, err
	}
	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPAddr:
			if addr := v.IP.To4(); addr != nil {
				return addr, nil, nil
			}
		case *net.IPNet:
			if addr := v.IP.To4(); addr != nil {
				return addr, v, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("could not find IPv4 address")
}

// FindHostByMAC returns the host record for the given MAC (case-insensitive).
// Returns (nil, nil) when no host is found.
func findHostByMAC(mac string) (*models.Host, error) {
    var host models.Host
    res := db.DB.
        Preload("Group").
        Preload("Pool").
        Where("LOWER(mac) = ?", strings.ToLower(mac)).
        First(&host)

    if res.Error != nil {
        if errors.Is(res.Error, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, res.Error
    }
	if host.Reimage == true {
    	return &host, nil
	} else {
		return nil, fmt.Errorf("%s is not flagged for re-imaging", host.Hostname)
	}
}

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
    if strings.HasPrefix(m, "/") {
        m = m[1:]
    }

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

func BroadcastAddress(gateway string, netmask string) (string, error) {
    ip := net.ParseIP(strings.TrimSpace(gateway)).To4()
    if ip == nil {
        return "", errors.New("invalid IPv4 gateway")
    }

    m := strings.TrimSpace(netmask)
    if strings.HasPrefix(m, "/") {
        m = m[1:]
    }

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

// NetmaskToCIDR converts a netmask in dotted-decimal ("255.255.255.0"),
// or CIDR formats ("24" or "/24") to the CIDR prefix length (e.g. 24).
func NetmaskToCIDR(maskStr string) (int, error) {
    m := strings.TrimSpace(maskStr)
    if strings.HasPrefix(m, "/") {
        m = strings.TrimPrefix(m, "/")
    }

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