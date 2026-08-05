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

package mgmtd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open3fs/m3fs/pkg/config"
)

func TestGenerateSingleReplicaChainFiles(t *testing.T) {
	stor := config.Storage{
		Nodes:            []string{"node1"},
		ReplicationFactor: 1,
		DiskNumPerNode:   1,
		TargetNumPerDisk: 2,
		TargetIDPrefix:   1,
		ChainIDPrefix:    9,
	}
	files := generateSingleReplicaChainFiles(stor)

	require.True(t, isSingleNodeSingleReplica(stor))
	require.Equal(t, "ChainId,TargetId\n"+
		"900100001,101000100101\n"+
		"900100002,101000100102\n", files.ChainsCSV)
	require.Equal(t, "ChainId\n"+
		"900100001\n"+
		"900100002\n", files.ChainTableCSV)

	lines := strings.Split(strings.TrimSpace(files.CreateTargetCmd), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], "create-target --node-id 10001 --disk-index 0 --target-id 101000100101 --chain-id 900100001")
	require.Contains(t, lines[1], "create-target --node-id 10001 --disk-index 0 --target-id 101000100102 --chain-id 900100002")
}

func TestCalcTargetAndChainID(t *testing.T) {
	// Match gen_chain_table.py formulas for prefix=1, node=10001, disk=0, target=0
	require.Equal(t, int64(101000100101), calcTargetID(1, 10001, 0, 0))
	require.Equal(t, int64(900100001), calcChainID(9, 0, 1))
}
