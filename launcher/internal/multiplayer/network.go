package multiplayer

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

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
			if e != nil || host.To4() == nil || host.IsLoopback() || host.IsUnspecified() || host.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, LocalAddress{host.String(), iface.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool {
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
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil || parsed.IsUnspecified() || parsed.IsLoopback() {
		return address, nil
	}
	locals, e := LocalAddresses()
	if e != nil {
		return "", e
	}
	for _, l := range locals {
		if net.ParseIP(l.IP).Equal(parsed) {
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
