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
	"time"

	"gorm.io/gorm"

	"github.com/open3fs/m3fs/pkg/errors"
)

// defines change plan step operation type
const (
	ChangePlanTypeAddStorNodes   = "add_stor_nodes"
	ChangePlanTypeAddFuseClients = "add_fuse_clients"
)

// ChangePlan is the model of change plan.
type ChangePlan struct {
	gorm.Model
	Data     string
	Type     string
	StartAt  *time.Time
	FinishAt *time.Time
}

// GetSteps returns steps of change plan.
func (c *ChangePlan) GetSteps(db *gorm.DB) ([]*ChangePlanStep, error) {
	var ret []*ChangePlanStep
	err := db.Model(new(ChangePlanStep)).Order("id ASC").Find(&ret, "change_plan_id = ?", c.ID).Error
	if err != nil {
		return nil, errors.Trace(err)
	}

	return ret, nil
}

// GetProcessingChangePlan return the processing change plan.
func GetProcessingChangePlan(db *gorm.DB) (*ChangePlan, error) {
	var changePlan ChangePlan
	err := db.Model(new(ChangePlan)).First(&changePlan, "finish_at IS NULL").Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errors.Trace(err)
	}

	return &changePlan, nil
}
