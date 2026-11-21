package multiplayer

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

// Resolve DNS only while dialing, where Connect already has a deadline.
func Address(host, port string) (string, error) {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		addr, err := netip.ParseAddr(host[1 : len(host)-1])
		if err != nil || !addr.Is6() {
			return "", fmt.Errorf("endereço IPv6 inválido")
		}
		host = addr.String()
	}
	if _, err := netip.ParseAddr(host); err != nil && !validServerName(host) {
		return "", fmt.Errorf("informe o IP ou nome DNS do servidor")
	}
	p, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil || p < 1 || p > 65535 {
		return "", fmt.Errorf("a porta precisa estar entre 1 e 65535")
	}
	return net.JoinHostPort(host, strconv.Itoa(p)), nil
}

func validServerName(host string) bool {
	name := strings.TrimSuffix(host, ".")
	if len(name) > 253 || strings.Trim(name, "0123456789.") == "" {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

type LocalAddress struct{ IP, Interface string }

// LocalAddresses includes VPN adapters; a Radmin address must not be treated
// as a public Internet endpoint or filtered out because it is not RFC1918.
func LocalAddresses() ([]LocalAddress, error) {
	interfaces, e := net.Interfaces()
	if e != nil {
		return nil, e
	}
	var out []LocalAddress
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, e := iface.Addrs()
		if e != nil {
			continue
		}
		for _, address := range addresses {
			host, _, e := net.ParseCIDR(address.String())
			if e != nil || host.IsLoopback() || host.IsUnspecified() || host.IsMulticast() || (host.To4() != nil && host.IsLinkLocalUnicast()) {
				continue
			}
			ip := host.String()
			if host.IsLinkLocalUnicast() {
				ip += "%" + iface.Name
			}
			out = append(out, LocalAddress{ip, iface.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		v4i, v4j := !strings.Contains(out[i].IP, ":"), !strings.Contains(out[j].IP, ":")
		if v4i != v4j {
			return v4i
		}
		return out[i].Interface < out[j].Interface || out[i].Interface == out[j].Interface && out[i].IP < out[j].IP
	})
	return out, nil
}
func BindAddress(ip, port string) (string, error) {
	if strings.TrimSpace(ip) == "" {
		ip = "0.0.0.0"
	}
	address, e := Address(ip, port)
	if e != nil {
		return "", e
	}
	host, _, _ := net.SplitHostPort(address)
	parsed, err := netip.ParseAddr(host)
	if err != nil {
		return "", fmt.Errorf("selecione o IP local para hospedar")
	}
	if parsed.IsUnspecified() || parsed.IsLoopback() {
		return address, nil
	}
	locals, e := LocalAddresses()
	if e != nil {
		return "", e
	}
	for _, l := range locals {
		local, err := netip.ParseAddr(l.IP)
		if err == nil && local.Unmap() == parsed.Unmap() {
			return address, nil
		}
	}
	return "", fmt.Errorf("o IP para hospedar precisa pertencer a este computador; selecione sua placa de rede ou Radmin")
}
func NetworkError(address string, err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("Falha ao conectar a %s: %v. Confira se o servidor está hospedado nesse IP/porta e se o firewall permite HyruleM.", address, err)
}
