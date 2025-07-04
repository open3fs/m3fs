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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/log"
)

// NewGormLogger returns a logger for gorm.
func NewGormLogger(logger log.Interface, slowThreshold time.Duration) gormlogger.Interface {
	return &GormLogger{
		logger:       logger,
		gormLevel:    gormlogger.Warn,
		infoStr:      "[gorm] ",
		warnStr:      "[gorm] ",
		errStr:       "[gorm] ",
		traceStr:     "[%.3fms] [rows:%v] %s",
		traceWarnStr: "%s \n[%.3fms] [rows:%v] %s",
		traceErrStr:  "%s \n[%.3fms] [rows:%v] %s",

		slowThreshold: slowThreshold,
	}
}

// GormLogger implements logger for GORM.
type GormLogger struct {
	logger                              log.Interface
	gormLevel                           gormlogger.LogLevel
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string

	slowThreshold time.Duration
}

// LogMode creates a new gorm logger with given log level.
func (gl *GormLogger) LogMode(gormLevel gormlogger.LogLevel) gormlogger.Interface {
	newGormLogger := *gl
	newGormLogger.gormLevel = gormLevel
	return &newGormLogger
}

// Info print info log.
func (gl *GormLogger) Info(ctx context.Context, msg string, args ...any) {
	if gl.gormLevel >= gormlogger.Info {
		// Info level of gormlogger is used to show debug information.
		gl.logger.Debugf(gl.infoStr+msg, args...)
	}
}

// Warn print warning log.
func (gl *GormLogger) Warn(ctx context.Context, msg string, args ...any) {
	if gl.gormLevel >= gormlogger.Warn {
		gl.logger.Warnf(gl.warnStr+msg, args...)
	}
}

// Error print error log.
func (gl *GormLogger) Error(ctx context.Context, msg string, args ...any) {
	if gl.gormLevel >= gormlogger.Error {
		gl.logger.Errorf(gl.errStr+msg, args...)
	}
}

// Trace print sql message.
func (gl *GormLogger) Trace(ctx context.Context, begin time.Time, f func() (string, int64), err error) {
	rowsStr := func(rows int64) string {
		if rows == -1 {
			return "-"
		}
		return strconv.FormatInt(rows, 10)
	}
	if gl.gormLevel > gormlogger.Silent {
		elapsed := time.Since(begin)
		switch {
		case err != nil && gl.gormLevel >= gormlogger.Error:
			if err == gorm.ErrRecordNotFound {
				return
			}
			sql, rows := f()
			gl.logger.Errorf(gl.traceErrStr,
				err, float64(elapsed.Milliseconds()), rowsStr(rows), sql)
		case elapsed > gl.slowThreshold && gl.slowThreshold != 0 && gl.gormLevel >= gormlogger.Warn:
			sql, rows := f()
			slowLog := fmt.Sprintf("SLOW SQL >= %v", gl.slowThreshold)
			gl.logger.Warnf(gl.traceWarnStr,
				slowLog, float64(elapsed.Milliseconds()), rowsStr(rows), sql)
		case gl.gormLevel == gormlogger.Info:
			sql, rows := f()
			gl.logger.Debugf(gl.traceStr,
				float64(elapsed.Milliseconds()), rowsStr(rows), sql)
		}
	}
}

// ConnectionArgs holds the connection arguments for the database.
type ConnectionArgs struct {
	Host             string
	Port             int
	User             string
	Password         string
	DBName           string
	SlowSQLThreshold time.Duration
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
	db, err := gorm.Open(postgres.Open(connArg.DSN()), &gorm.Config{
		Logger: NewGormLogger(log.Logger, connArg.SlowSQLThreshold),
	})
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
		new(StorService),
		new(Disk),
		new(Target),
		new(Chain),
		new(FuseClient),
		new(ChangePlan),
		new(ChangePlanStep),
	)
	if err != nil {
		return errors.Annotatef(err, "create resource tables")
	}

	return nil
}
