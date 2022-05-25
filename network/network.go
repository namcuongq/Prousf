package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

type physicalInterface struct {
	DstTest string
}

type WindowsRouter struct {
	Destination string
	Netmask     string
	Gateway     string
	Interface   string
	Metric      string
}

type PacketHeader struct {
	IPSrc    net.IP
	IPDst    net.IP
	IsIPv6   bool
	Protocol string
}

func ParseHeaderPacket(buf []byte) PacketHeader {
	var ipHeader PacketHeader
	switch buf[0] & 0xF0 {
	case 0x40:
		ipHeader.IPSrc = net.IP(buf[12:16])
		ipHeader.IPDst = net.IP(buf[16:20])
		ipHeader.Protocol = fmt.Sprintf("%d", buf[9])
	case 0x60:
		ipHeader.IsIPv6 = true
		ipHeader.IPSrc = net.IP(buf[8:24])
		ipHeader.IPDst = net.IP(buf[24:40])
		ipHeader.Protocol = fmt.Sprintf("%d", buf[7])
	}

	// switch buf[0] & 0xF0 {
	// case 0x40:
	// 	fmt.Println("received ipv4")
	// 	fmt.Printf("Length: %d\n", binary.BigEndian.Uint16(buf[2:4]))
	// 	fmt.Printf("Protocol: %d (1=ICMP, 6=TCP, 17=UDP)\n", buf[9])
	// 	fmt.Printf("Source IP: %s\n", net.IP(buf[12:16]))
	// 	fmt.Printf("Destination IP: %s\n", net.IP(buf[16:20]))
	// case 0x60:
	// 	fmt.Println("received ipv6")
	// 	fmt.Printf("Length: %d\n", binary.BigEndian.Uint16(buf[4:6]))
	// 	fmt.Printf("Protocol: %d (1=ICMP, 6=TCP, 17=UDP)\n", buf[7])
	// 	fmt.Printf("Source IP: %s\n", net.IP(buf[8:24]))
	// 	fmt.Printf("Destination IP: %s\n", net.IP(buf[24:40]))
	// }

	return ipHeader
}

func GetDefaultGatewayWindows() (WindowsRouter, error) {
	var route = WindowsRouter{}
	routeCmd := exec.Command("route", "print", "0.0.0.0")
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return route, fmt.Errorf("get default gateway err: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	sep := 0
	for idx, line := range lines {
		if sep == 3 {
			if len(lines) <= idx+2 {
				return route, fmt.Errorf("get default gateway err: no gateway")
			}

			fields := strings.Fields(lines[idx+2])
			if len(fields) < 5 {
				return route, fmt.Errorf("get default gateway err: can't parse")
			}

			route = WindowsRouter{
				Destination: fields[0],
				Netmask:     fields[1],
				Gateway:     fields[2],
				Interface:   fields[3],
				Metric:      fields[4],
			}
			break
		}
		if strings.HasPrefix(line, "=======") {
			sep++
			continue
		}
	}

	return route, nil

}

func FindPhysicalInterface(DstTest string) (net.Interface, error) {
	var p physicalInterface
	p.DstTest = DstTest
	return p.findPhysicalInterface()
}

func (p physicalInterface) findPhysicalInterface() (net.Interface, error) {
	var internetInterface net.Interface
	outboundIP, err := p.getOutboundIP()
	if err != nil {
		return internetInterface, fmt.Errorf("get outbound ip err: %v", err)
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return internetInterface, fmt.Errorf("get list interface err: %v", err)
	}

L:
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return internetInterface, fmt.Errorf("get list address err: %v", err)
		}

		for numberAddr, addr := range addrs {
			if outboundIP == addr.String() {
				if p.sendReqTest(&i, numberAddr) {
					internetInterface = i
					break L
				}
				break
			}
		}
	}

	return internetInterface, nil
}

func (p physicalInterface) sendReqTest(ief *net.Interface, numberAddr int) bool {
	addrs, err := ief.Addrs()
	if err != nil {
		return false
	}
	tcpAddr := &net.TCPAddr{
		IP: addrs[numberAddr].(*net.IPNet).IP,
	}

	d := net.Dialer{LocalAddr: tcpAddr, Timeout: time.Duration(3) * time.Second}
	_, err = d.Dial("tcp", p.DstTest)
	if err != nil {
		return false
	}
	return true
}

func (p physicalInterface) getOutboundIP() (string, error) {
	conn, err := net.Dial("tcp", p.DstTest)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.TCPAddr)
	sz, _ := net.IPMask(localAddr.IP.DefaultMask()).Size()
	return fmt.Sprintf("%v/%v", net.IP(localAddr.IP), sz), nil
}

func CIDRToMask(ip string) string {
	_, ipv4Net, _ := net.ParseCIDR(ip)
	return ipv4MaskString(ipv4Net.Mask)
}

func GetIp(str string) string {
	if strings.Contains(str, ":") {
		return str[:strings.Index(str, ":")]
	}
	return str[:strings.Index(str, "/")]
}

func ipv4MaskString(m []byte) string {
	if len(m) != 4 {
		panic("ipv4Mask: len must be 4 bytes")
	}

	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}
