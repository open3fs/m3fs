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
	"math/rand"
	"os"
	"strconv"
	"sync/atomic"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/pg/model"
)

var (
	dbTemplateCreated int64
	defaultDB         *gorm.DB
	dbTemplate        string
)

// ConnectionArgs returns a connetion argument to test database.
func ConnectionArgs() *model.ConnectionArgs {
	connArg := &model.ConnectionArgs{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "password",
		DBName:   "postgres",
	}
	setConnArgByEnv(connArg)

	return connArg
}

func setConnArgByEnv(connArg *model.ConnectionArgs) {
	pgHost := os.Getenv("M3FS_TEST_PG_HOST")
	if pgHost != "" {
		connArg.Host = pgHost
	}
	pgPort := os.Getenv("M3FS_TEST_PG_PORT")
	if pgPort != "" {
		port, err := strconv.ParseUint(pgPort, 10, 16)
		if err == nil {
			connArg.Port = int(port)
		}
	}
	pgUser := os.Getenv("M3FS_TEST_PG_USER")
	if pgUser != "" {
		connArg.User = pgUser
	}
	pgPassword := os.Getenv("M3FS_TEST_PG_PASSWORD")
	if pgPassword != "" {
		connArg.Password = pgPassword
	}
	pgDBName := os.Getenv("M3FS_TEST_PG_DB_NAME")
	if pgDBName != "" {
		connArg.DBName = pgDBName
	}
}

func closeDB(db *gorm.DB) (err error) {
	sqldb, e := db.DB()
	if e != nil {
		return errors.Trace(e)
	}
	if err = sqldb.Close(); err != nil {
		return errors.Annotate(err, "close db")
	}

	return nil
}

// createTemplateDB creates a database as template database which will be used
// to create other test databases.
func createTemplateDB(db *gorm.DB, arg *model.ConnectionArgs) (err error) {
	created := atomic.LoadInt64(&dbTemplateCreated)
	if created != 0 {
		return nil
	}

	dbTemplate = fmt.Sprintf("data_tmpl_%d", rand.Int31())
	if err = db.Exec("CREATE DATABASE ?", clause.Table{Name: dbTemplate}).Error; err != nil {
		return errors.Trace(err)
	}

	arg.DBName = dbTemplate
	newDSN := arg.DSN()
	newDB, err := gorm.Open(postgres.Open(newDSN),
		&gorm.Config{},
	)
	if err != nil {
		return errors.Annotatef(err, "connect to %s", newDSN)
	}
	defer func() {
		if e := closeDB(newDB); e != nil {
			log.Logger.Errorf("Failed to close db: %s", e)
		}
	}()

	if err = model.SyncTables(newDB); err != nil {
		return errors.Annotate(err, "sync tables")
	}

	atomic.StoreInt64(&dbTemplateCreated, 1)
	log.Logger.Infof("Template database %s created", dbTemplate)

	return nil
}

// CreateTestDB create a test database from template for a suite.
func CreateTestDB(createTables bool) (dbName string, err error) {
	created := atomic.LoadInt64(&dbTemplateCreated)
	if created == 0 {
		return "", fmt.Errorf("template database does not exist")
	}

	dbName = fmt.Sprintf("data_db_%d", rand.Int63())
	if createTables {
		err = defaultDB.Exec("CREATE DATABASE ? TEMPLATE = ?",
			clause.Table{Name: dbName}, clause.Table{Name: dbTemplate}).Error
		if err != nil {
			return "", err
		}
	} else {
		err = defaultDB.Exec("CREATE DATABASE ?", clause.Table{Name: dbName}).Error
		if err != nil {
			return "", err
		}
	}
	log.Logger.Infof("Test database %s created", dbName)

	return dbName, nil
}

// DeleteTestDB delete a test database.
func DeleteTestDB(dbName string, db *gorm.DB) (err error) {
	if err = model.CloseDB(db); err != nil {
		return errors.Trace(err)
	}

	err = defaultDB.Exec("DROP DATABASE ?", clause.Table{Name: dbName}).Error
	if err != nil {
		return errors.Annotatef(err, "drop database %s", dbName)
	}

	return nil
}

// SetupEnv setups test environment about database.
func SetupEnv() (err error) {
	arg := ConnectionArgs()
	dsn := arg.DSN()

	defaultDB, err = gorm.Open(postgres.Open(dsn),
		&gorm.Config{},
	)
	if err != nil {
		return errors.Annotatef(err, "connect to %s", dsn)
	}

	if err = createTemplateDB(defaultDB, arg); err != nil {
		return errors.Annotate(err, "create template database")
	}

	return nil
}
