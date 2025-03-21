// Copyright 2025 Open3FS Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"net"

	"github.com/open3fs/m3fs/pkg/errors"
)

// GetLocalIPs returns all local IP addresses.
func GetLocalIPs() ([]*net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	var ips []*net.IP
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			ips = append(ips, &ipNet.IP)
		}
	}

	return ips, nil
}

// IsLocalHost checks if the given host is a local host.
func IsLocalHost(host string, localIPs []*net.IP) (bool, error) {
	hostIPs, err := net.LookupIP(host)
	if err != nil {
		return false, errors.Annotatef(err, "lookup IP address of %s", host)
	}

	for _, ip := range hostIPs {
		for _, localIP := range localIPs {
			if ip.Equal(*localIP) {
				return true, nil
			}
		}
	}

	return false, nil
}

// GenerateIPRange generates a list of IP addresses in the range of [ipStart, ipEnd].
func GenerateIPRange(ipStart, ipEnd string) ([]string, error) {
	start := net.ParseIP(ipStart)
	if start == nil {
		return nil, errors.Errorf("invalid start IP: %s", ipStart)
	}
	end := net.ParseIP(ipEnd)
	if end == nil {
		return nil, errors.Errorf("invalid end IP: %s", ipEnd)
	}

	if (start.To4() == nil) != (end.To4() == nil) {
		return nil, errors.Errorf("start IP and end IP are not in the same format")
	}

	startInt, endInt := uint64(0), uint64(0)
	ipv4 := start.To4() != nil
	if ipv4 {
		// ipv4
		startInt = ipToInt(start)
		endInt = ipToInt(end)
	} else {
		// ipv6
		startInt = ipv6ToInt(start)
		endInt = ipv6ToInt(end)
	}
	if startInt > endInt {
		return nil, errors.Errorf("start IP %s is greater than end IP %s", ipStart, ipEnd)
	}

	ips := make([]string, 0, endInt-startInt+1)
	for i := startInt; i <= endInt; i++ {
		var ip net.IP
		if ipv4 {
			ip = intToIP(uint32(i))
		} else {
			ip = intToIPv6(i)
		}
		ips = append(ips, ip.String())
	}

	return ips, nil
}

func ipToInt(ip net.IP) uint64 {
	ip = ip.To4()
	return uint64(uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3]))
}

func ipv6ToInt(ip net.IP) uint64 {
	ip = ip.To16()
	return uint64(ip[0])<<56 | uint64(ip[1])<<48 | uint64(ip[2])<<40 | uint64(ip[3])<<32 |
		uint64(ip[4])<<24 | uint64(ip[5])<<16 | uint64(ip[6])<<8 | uint64(ip[7])
}

// intToIP 将32位整数转换为IPv4地址
func intToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	ip[0] = byte(n >> 24)
	ip[1] = byte(n >> 16)
	ip[2] = byte(n >> 8)
	ip[3] = byte(n)
	return ip
}

func intToIPv6(n uint64) net.IP {
	ip := make(net.IP, 16)
	ip[0] = byte(n >> 56)
	ip[1] = byte(n >> 48)
	ip[2] = byte(n >> 40)
	ip[3] = byte(n >> 32)
	ip[4] = byte(n >> 24)
	ip[5] = byte(n >> 16)
	ip[6] = byte(n >> 8)
	ip[7] = byte(n)
	return ip
}
