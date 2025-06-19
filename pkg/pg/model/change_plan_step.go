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

package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ChangePlanStepOpType defines ChangePlanStep operation type
var ChangePlanStepOpType = struct {
	CreateStorService            string
	OfflineTarget                string
	RemoveTarget                 string
	RemoveTargetFromChain        string
	CreateTarget                 string
	AddTargetToChain             string
	UploadChains                 string
	UploadChainTable             string
	CreateNewChainAndTargetModel string
}{
	CreateStorService:            "create_stor_service",
	OfflineTarget:                "offline_target",
	RemoveTarget:                 "remove_target",
	RemoveTargetFromChain:        "remove_target_from_chain",
	CreateTarget:                 "create_target",
	AddTargetToChain:             "add_target_to_chain",
	UploadChains:                 "upload_chains",
	UploadChainTable:             "upload_chain_table",
	CreateNewChainAndTargetModel: "create_new_chain_and_target_model",
}

// ChangePlanStep is the model of change plan step.
type ChangePlanStep struct {
	gorm.Model
	ChangePlanID  uint
	OperationType string
	OperationData string
	StartAt       *time.Time
	FinishAt      *time.Time
}

// String return string format of change plan step
func (s *ChangePlanStep) String() string {
	return fmt.Sprintf("ChangePlanStep(ID=%d,OperationType=%s)", s.ID, s.OperationType)
}
