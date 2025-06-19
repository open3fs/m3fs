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

package steps

import (
	"bufio"
	"context"
	"strings"
	"time"

	"github.com/open3fs/m3fs/pkg/errors"
)

// WaitServiceState wait service state
func WaitServiceState(
	ctx context.Context, listNodesFunc func(context.Context) (string, error),
	serviceID, targetState string, timeout time.Duration) error {

	timer := time.NewTimer(timeout)
	defer timer.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ctx.Done():
			break loop
		default:
			nodesInfo, err := listNodesFunc(ctx)
			if err != nil {
				return errors.Trace(err)
			}
			//nolint:lll
			// output of list nodes:
			// Id     Type     Status               Hostname  Pid  Tags  LastHeartbeatTime    ConfigVersion  ReleaseVersion
			// 1      MGMTD    PRIMARY_MGMTD        fook-1    1    []    N/A                  1(UPTODATE)    250228-dev-1-999999-ee9a5cee
			scanner := bufio.NewScanner(strings.NewReader(nodesInfo))
			scanner.Scan()
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Fields(line)
				if len(parts) < 9 {
					return errors.Errorf("invalid list-nodes output: %s", line)
				}
				if parts[0] != serviceID {
					continue
				}
				if parts[2] != targetState {
					time.Sleep(time.Second)
					continue
				}
				return nil
			}
		}
	}

	return errors.Errorf("timeout to wait service %s to %s", serviceID, targetState)
}
