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
