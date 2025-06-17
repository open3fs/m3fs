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
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/open3fs/m3fs/pkg/errors"
)

// ConnectionArgs holds the connection arguments for the database.
type ConnectionArgs struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// String returns a string representation of the connection arguments.
func (arg *ConnectionArgs) String() string {
	return fmt.Sprintf("%s:%d", arg.Host, arg.Port)
}

// DSN generates a Data Source Name (DSN) for the PostgreSQL connection.
func (arg *ConnectionArgs) DSN() string {
	newPart := func(k string, v any) string {
		return fmt.Sprintf("%s=%v ", k, v)
	}
	parts := []string{
		newPart("host", arg.Host),
		newPart("port", arg.Port),
		newPart("user", arg.User),
		newPart("password", arg.Password),
		newPart("dbname", arg.DBName),
		newPart("sslmode", "disable"),
	}

	return strings.Join(parts, " ")
}

// NewDB creates a new database connection using the provided connection arguments.
func NewDB(connArg *ConnectionArgs) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(connArg.DSN()), &gorm.Config{})
	if err != nil {
		return nil, errors.Annotate(err, "open database")
	}

	return db, nil
}

// CloseDB closes the database connection.
func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return errors.Annotate(err, "get sql DB")
	}

	if err := sqlDB.Close(); err != nil {
		return errors.Annotate(err, "close sql DB")
	}

	return nil
}

// SyncTables create/update resource tables
func SyncTables(db *gorm.DB) error {
	err := db.AutoMigrate(
		new(Cluster),
		new(Node),
		new(FdbCluster),
		new(FdbProcess),
		new(ChService),
		new(GrafanaService),
		new(PgService),
		new(MgmtService),
		new(MonService),
		new(MetaService),
		new(StorageService),
		new(Disk),
		new(Target),
		new(Chain),
		new(FuseClient),
	)
	if err != nil {
		return errors.Annotatef(err, "create resource tables")
	}

	return nil
}
