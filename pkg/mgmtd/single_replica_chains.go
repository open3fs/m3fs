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
	"fmt"
	"strings"

	"github.com/open3fs/m3fs/pkg/config"
)

// isSingleNodeSingleReplica reports whether chain placement can be generated
// without data_placement.py (1 storage node + RF=1).
func isSingleNodeSingleReplica(stor config.Storage) bool {
	return len(stor.Nodes) == 1 && stor.ReplicationFactor == 1
}

// calcTargetID mirrors deploy/data_placement/src/setup/gen_chain_table.py::calc_target_id
func calcTargetID(targetIDPrefix, nodeID, diskIndex, targetIndex int64) int64 {
	return ((targetIDPrefix*1_000_000+nodeID)*1_000+(diskIndex+1))*100 + (targetIndex + 1)
}

// calcChainID mirrors gen_chain_table.py chain id encoding for CR.
func calcChainID(chainIDPrefix, diskIndex, chainIndex int64) int64 {
	return (chainIDPrefix*1_000+(diskIndex+1))*100_000 + chainIndex
}

type singleReplicaChainFiles struct {
	ChainsCSV      string
	ChainTableCSV  string
	CreateTargetCmd string
}

// generateSingleReplicaChainFiles builds the same three output files that
// data_placement.py + gen_chain_table.py would produce for 1 node / RF=1.
// Each target is its own chain (group size = 1).
func generateSingleReplicaChainFiles(stor config.Storage) singleReplicaChainFiles {
	nodeID := int64(10001)
	targetPrefix := stor.TargetIDPrefix
	chainPrefix := stor.ChainIDPrefix
	numDisks := stor.DiskNumPerNode
	numTargets := stor.TargetNumPerDisk

	var chainsCSV strings.Builder
	var tableCSV strings.Builder
	var cmds strings.Builder

	chainsCSV.WriteString("ChainId,TargetId\n")
	tableCSV.WriteString("ChainId\n")

	for diskIndex := 0; diskIndex < numDisks; diskIndex++ {
		for targetIndex := 0; targetIndex < numTargets; targetIndex++ {
			// For RF=1, group id / chain_index is 1..numTargets (same as trivial incidence).
			chainIndex := int64(targetIndex + 1)
			targetID := calcTargetID(targetPrefix, nodeID, int64(diskIndex), int64(targetIndex))
			chainID := calcChainID(chainPrefix, int64(diskIndex), chainIndex)

			fmt.Fprintf(&chainsCSV, "%d,%d\n", chainID, targetID)
			fmt.Fprintf(&tableCSV, "%d\n", chainID)
			fmt.Fprintf(&cmds,
				"create-target --node-id %d --disk-index %d --target-id %d --chain-id %d --use-new-chunk-engine\n",
				nodeID, diskIndex, targetID, chainID)
		}
	}

	return singleReplicaChainFiles{
		ChainsCSV:       chainsCSV.String(),
		ChainTableCSV:   tableCSV.String(),
		CreateTargetCmd: cmds.String(),
	}
}
