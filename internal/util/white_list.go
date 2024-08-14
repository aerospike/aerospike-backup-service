package util

import (
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
)

type IPWhiteList struct {
	addresses map[string]*netip.Addr
	networks  []*netip.Prefix
	allowAny  bool
}

func NewIPWhiteList(ipList []string) *IPWhiteList {
	addresses := make(map[string]*netip.Addr)
	networks := make([]*netip.Prefix, 0)
	var allowAny bool
	for _, ip := range ipList {
		if strings.HasPrefix(ip, "0.0.0.0") {
			allowAny = true
		}
		network, err := netip.ParsePrefix(ip)
		if err != nil {
			ipAddr, err := netip.ParseAddr(ip)
			if err != nil {
				panic(fmt.Sprintf("invalid ip configuration: %s", ip))
			}
			addresses[ip] = &ipAddr
		} else {
			networks = append(networks, &network)
		}
	}
	return &IPWhiteList{
		addresses: addresses,
		networks:  networks,
		allowAny:  allowAny,
	}
}

func (wl *IPWhiteList) IsAllowed(ip string) bool {
	if wl.allowAny {
		return true
	}
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		slog.Warn("Invalid client ip", "ip", ip)
		return false
	}
	_, ok := wl.addresses[ip]
	if ok {
		return true
	}

	for _, network := range wl.networks {
		if network.Contains(ipAddr) {
			return true
		}
	}

	return false
}
