package lookup

import (
	"fmt"
	"net"
	"strings"
)

// Make a connection to testUrl to infer the local network address. Returns
// the address of the interface that was used for the connection.
func LocalAddress(testUrl string) (host string, err error) {
	if strings.HasPrefix(testUrl, "http://") {
		testUrl = testUrl[7:]
	} else if strings.HasPrefix(testUrl, "https://") {
		testUrl = testUrl[8:]
	}
	if !strings.ContainsRune(testUrl, ':') {
		testUrl = testUrl + ":80"
	}
	if conn, err := net.Dial("udp", fmt.Sprintf("%s", testUrl)); err != nil {
		return "", err
	} else {
		defer conn.Close()
		host = strings.Split(conn.LocalAddr().String(), ":")[0]
	}
	return
}

// List all IP addresses found for the named interface
func InterfaceIPs(interfaceName string) (ips []net.IP, err error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return
	}
	ips = make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
			ips = append(ips, ip)
		}
	}
	return
}

// List the first IP address found for the named interface
func InterfaceIP(interfaceName string) (ip net.IP, err error) {
	ips, err := InterfaceIPs(interfaceName)
	if err != nil {
		return
	}
	return ips[0], nil
}

// List the first IPv4 address found for the named interface
func InterfaceIPv4(interfaceName string) (ip net.IP, err error) {
	ips, err := InterfaceIPs(interfaceName)
	if err != nil {
		return
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip, nil
		}
	}
	return
}

// List all interfaces matching the given flags. Or all interfaces if no flags
// are passed. E.g. FilterInterfaces(net.FlagUp, net.FlagBroadcast)
func FilterInterfaces(f ...net.Flags) (interfaces []net.Interface, err error) {
	allInterfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	interfaces = make([]net.Interface, 0, len(allInterfaces))
	for _, iface := range allInterfaces {
		include := true
		for _, flag := range f {
			if iface.Flags&flag == 0 {
				include = false
			}
		}
		if include {
			interfaces = append(interfaces, iface)
		}
	}
	return
}

// Get the first address found for the named interface, optionally returning
// only IPv4 addresses.
func GetInterfaceAddress(name string, filterIPv4 bool) (address string, err error) {
	var ip net.IP
	if filterIPv4 {
		if ip, err = InterfaceIPv4(name); err != nil {
			return
		}
	} else {
		if ip, err = InterfaceIP(name); err != nil {
			return
		}
	}
	return ip.String(), nil
}

// Get the first address found for the first broadcast interface, optionally returning
// only IPv4 addresses.
func GetAddress(filterIPv4 bool) (address string, err error) {
	interfaces, err := FilterInterfaces(net.FlagUp, net.FlagBroadcast)
	if err != nil {
		return
	}
	address, err = GetInterfaceAddress(interfaces[0].Name, filterIPv4)
	return
}

// FindOpenTCPAddress finds an available TCP address on the given interface. It does this by binding to
// <addr>:0, retrieving the resolved address, and then closing the connection.
func FindOpenTCPAddress(name string, filterIPv4 bool) (addr net.Addr, err error) {
	var address string
	if name != "" {
		if address, err = GetInterfaceAddress(name, filterIPv4); err != nil {
			return
		}
	} else {
		if address, err = GetAddress(filterIPv4); err != nil {
			return
		}
	}
	resolvedAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(address, "0"))
	if err != nil {
		return
	}
	if listener, err := net.ListenTCP("tcp", resolvedAddr); err != nil {
		return nil, err
	} else {
		defer listener.Close()
		addr = listener.Addr()
	}
	return
}

// FindOpenUDPAddress finds an available UDP address on the given interface. It does this by binding to
// <addr>:0, retrieving the resolved address, and then closing the connection.
func FindOpenUDPAddress(name string, filterIPv4 bool) (addr net.Addr, err error) {
	var address string
	if name != "" {
		if address, err = GetInterfaceAddress(name, filterIPv4); err != nil {
			return
		}
	} else {
		if address, err = GetAddress(filterIPv4); err != nil {
			return
		}
	}
	resolvedAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(address, "0"))
	if err != nil {
		return
	}
	if listener, err := net.ListenUDP("udp", resolvedAddr); err != nil {
		return nil, err
	} else {
		defer listener.Close()
		addr = listener.LocalAddr()
	}
	return
}

// FindOpenTCPPort uses FindOpenTCPAddress to find an available TCP address and then return the port
func FindOpenTCPPort(name string, filterIPv4 bool) (port int, err error) {
	if addr, err := FindOpenTCPAddress(name, filterIPv4); err != nil {
		return 0, err
	} else {
		port = addr.(*net.TCPAddr).Port
	}
	return
}

// FindOpenUDPPort uses FindOpenUDPAddress to find an available UDP address and then return the port
func FindOpenUDPPort(name string, filterIPv4 bool) (port int, err error) {
	if addr, err := FindOpenUDPAddress(name, filterIPv4); err != nil {
		return 0, err
	} else {
		port = addr.(*net.UDPAddr).Port
	}
	return
}
