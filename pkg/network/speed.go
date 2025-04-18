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

package network

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	networkTimeout = 5 * time.Second
)

var (
	ibSpeedPattern  = regexp.MustCompile(`rate:\s+(\d+)\s+Gb/sec`)
	ethSpeedPattern = regexp.MustCompile(`Speed:\s+(\d+)\s*([GMK]b/?s)`)
)

// GetNetworkSpeed returns the network speed for the given network type
func GetNetworkSpeed(networkType string) string {
	if speed := getIBNetworkSpeed(); speed != "" {
		return speed
	}

	if speed := getEthernetSpeed(); speed != "" {
		return speed
	}

	return getDefaultNetworkSpeed(networkType)
}

// getDefaultNetworkSpeed returns the default network speed
func getDefaultNetworkSpeed(networkType string) string {
	switch networkType {
	case "ib":
		return "50 Gb/sec"
	case "rdma":
		return "100 Gb/sec"
	default:
		return "10 Gb/sec"
	}
}

// getIBNetworkSpeed returns the InfiniBand network speed
func getIBNetworkSpeed() string {
	ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
	defer cancel()

	cmdIB := exec.CommandContext(ctx, "ibstatus")
	output, err := cmdIB.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logrus.Error("Timeout while getting IB network speed")
		} else {
			logrus.Debugf("Failed to get IB network speed: %v", err)
		}
		return ""
	}

	matches := ibSpeedPattern.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		return matches[1] + " Gb/sec"
	}

	logrus.Debug("No IB network speed found in ibstatus output")
	return ""
}

// getEthernetSpeed returns the Ethernet network speed
func getEthernetSpeed() string {
	interfaceName, err := getDefaultInterface()
	if err != nil {
		logrus.Debugf("Failed to get default interface: %v", err)
		return ""
	}
	if interfaceName == "" {
		logrus.Debug("No default interface found")
		return ""
	}

	speed := getInterfaceSpeed(interfaceName)
	if speed == "" {
		logrus.Debugf("Failed to get speed for interface %s", interfaceName)
	}
	return speed
}

// getDefaultInterface returns the default network interface
func getDefaultInterface() (string, error) {
	cmdIp := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}'")
	interfaceOutput, err := cmdIp.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get default interface: %w", err)
	}

	interfaceName := strings.TrimSpace(string(interfaceOutput))
	if interfaceName == "" {
		return "", fmt.Errorf("no default interface found")
	}
	return interfaceName, nil
}

// getInterfaceSpeed returns the speed of a network interface
func getInterfaceSpeed(interfaceName string) string {
	if interfaceName == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
	defer cancel()

	cmdEthtool := exec.CommandContext(ctx, "ethtool", interfaceName)
	ethtoolOutput, err := cmdEthtool.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logrus.Error("Timeout while getting interface speed")
		} else {
			logrus.Debugf("Failed to get interface speed for %s: %v", interfaceName, err)
		}
		return ""
	}

	matches := ethSpeedPattern.FindStringSubmatch(string(ethtoolOutput))
	if len(matches) > 2 {
		return matches[1] + " " + matches[2]
	}

	logrus.Debugf("No speed found in ethtool output for interface %s", interfaceName)
	return ""
}
