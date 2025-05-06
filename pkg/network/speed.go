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
	"regexp"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
)

var (
	ibSpeedPattern  = regexp.MustCompile(`rate:\s+(\d+)\s+Gb/sec`)
	ethSpeedPattern = regexp.MustCompile(`Speed:\s+(\d+)\s*([GMK]b/?s)`)
)

// GetSpeed returns the speed of the network interface
func GetSpeed(ctx context.Context, runner external.RunnerInterface, networkType config.NetworkType) string {

	if networkType == config.NetworkTypeIB {
		speed, _ := getIBNetworkSpeed(ctx, runner)
		if speed != "" {
			return speed
		}
	}

	speed, _ := getEthernetSpeed(ctx, runner)
	if speed != "" {
		return speed
	}

	if speed == "" || speed == "Unknown!" {
		switch networkType {
		case config.NetworkTypeIB:
			speed = "50 Gbps"
		case config.NetworkTypeRDMA:
			speed = "100 Gbps"
		default:
			speed = "10 Gbps"
		}
	}

	return speed
}

func getIBNetworkSpeed(ctx context.Context, runner external.RunnerInterface) (string, error) {
	output, err := runner.Exec(ctx, "ibstatus")
	if err != nil {
		return "", errors.Annotate(err, "ibstatus")
	}

	matches := ibSpeedPattern.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1] + " Gb/sec", nil
	}

	return "", nil
}

// getEthernetSpeed returns the Ethernet network speed
func getEthernetSpeed(ctx context.Context, runner external.RunnerInterface) (string, error) {
	interfaceName, err := getDefaultInterface(ctx, runner)
	if err != nil {
		return "", errors.Annotate(err, "get default interface")
	}
	if interfaceName == "" {
		return "", errors.New("no default interface found")
	}

	speed, err := getInterfaceSpeed(ctx, runner, interfaceName)
	if err != nil {
		return "", errors.Annotatef(err, "get interface speed for %s", interfaceName)
	}
	return speed, nil
}

// getDefaultInterface returns the default network interface
func getDefaultInterface(ctx context.Context, runner external.RunnerInterface) (string, error) {
	output, err := runner.Exec(ctx, "ip route | grep default | awk '{print $5}'")
	if err != nil {
		return "", errors.Annotate(err, "failed to get default interface")
	}

	interfaceName := strings.TrimSpace(output)
	if interfaceName == "" {
		return "", errors.New("no default interface found")
	}
	return interfaceName, nil
}

// getInterfaceSpeed returns the speed of a network interface
func getInterfaceSpeed(ctx context.Context, runner external.RunnerInterface, interfaceName string) (string, error) {
	if interfaceName == "" {
		return "", errors.New("empty interface name")
	}

	output, err := runner.Exec(ctx, "ethtool", interfaceName)
	if err != nil {
		return "", errors.Annotatef(err, "failed to get interface speed for %s", interfaceName)
	}

	matches := ethSpeedPattern.FindStringSubmatch(output)
	if len(matches) > 2 {
		return strings.TrimSpace(matches[1] + " " + matches[2]), nil
	}

	return "", errors.Errorf("no speed found in ethtool output for interface %s", interfaceName)
}
