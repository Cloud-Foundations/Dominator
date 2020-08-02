package util

import (
	"net"
)

const (
	RouteFlagUp = 1 << iota
	RouteFlagGateway
	RouteFlagHost
)

type DefaultRouteInfo struct {
	Address   net.IP
	Interface string
	Mask      net.IPMask
}

type RouteEntry struct {
	BaseAddr      net.IP
	BroadcastAddr net.IP
	Flags         uint32
	GatewayAddr   net.IP
	InterfaceName string
	Mask          net.IPMask
}

type RouteTable struct {
	DefaultRoute *DefaultRouteInfo
	RouteEntries []*RouteEntry
}

type ResolverConfiguration struct {
	Domain        string
	Nameservers   []net.IP
	SearchDomains []string
}

func CopyIP(ip net.IP) net.IP {
	return copyIP(ip)
}

func DecrementIP(ip net.IP) {
	decrementIP(ip)
}
func GetDefaultRoute() (*DefaultRouteInfo, error) {
	return getDefaultRoute()
}

func GetMyIP() (net.IP, error) {
	return getMyIP()
}

func GetResolverConfiguration() (*ResolverConfiguration, error) {
	return getResolverConfiguration()
}

func GetRouteTable() (*RouteTable, error) {
	return getRouteTable()
}

func IncrementIP(ip net.IP) {
	incrementIP(ip)
}

func InvertIP(input net.IP) {
	invertIP(input)
}

func ShrinkIP(netIP net.IP) net.IP {
	return shrinkIP(netIP)
}
