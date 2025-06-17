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
	"gorm.io/gorm"

	"github.com/open3fs/m3fs/pkg/pg/model"
	tbase "github.com/open3fs/m3fs/tests/base"
)

// Suite is the base suite for all suites need model function.
type Suite struct {
	tbase.Suite

	dbName       string
	CreateTables bool
	db           *gorm.DB
}

// SetupSuite runs before suite case run.
func (s *Suite) SetupSuite() {
	s.Suite.SetupSuite()
	s.NoError(SetupEnv())
	s.CreateTables = true
}

// SetupTest runs before each case.
func (s *Suite) SetupTest() {
	s.Suite.SetupTest()

	dbName, err := CreateTestDB(s.CreateTables)
	s.NoError(err)
	s.dbName = dbName
	s.db, err = model.NewDB(s.ConnectionArgs())
	s.NoError(err)
}

// TearDownTest runs after each case.
func (s *Suite) TearDownTest() {
	s.NoError(closeDB(s.db))
	s.NoError(DeleteTestDB(s.dbName, s.NewDB()))
	s.Suite.TearDownTest()
}

// ConnectionArgs return db connection args
func (s *Suite) ConnectionArgs() *model.ConnectionArgs {
	connArg := ConnectionArgs()
	connArg.DBName = s.dbName
	return connArg
}

// NewDB creates a db object
func (s *Suite) NewDB() *gorm.DB {
	return s.db.Session(&gorm.Session{}).WithContext(s.Ctx())
}
