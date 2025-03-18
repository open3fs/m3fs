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

package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

// defines logger field keys.
const (
	FieldKeyNode = "NODE"
	FieldKeyTask = "TASK"
	FieldKeyStep = "STEP"
)

// Interface is the interface of logger.
type Interface interface {
	Subscribe(key, val string) Interface

	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Warningf(format string, args ...any)
	Errorf(format string, args ...any)

	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Warning(args ...any)
	Error(args ...any)

	Debugln(args ...any)
	Infoln(args ...any)
	Warnln(args ...any)
	Warningln(args ...any)
	Errorln(args ...any)
}

var _ Interface = new(logger)

// Logger is the global logger.
var Logger Interface

// Logger is the management unit of logging functions.
type logger struct {
	*logrus.Logger
	fields map[string]any
}

// Debugf logs a message at level Debug on the standard logger.
func (l *logger) Debugf(format string, args ...any) {
	l.WithFields(l.fields).Debugf(format, args...)
}

// Infof logs a message at level Info on the standard logger.
func (l *logger) Infof(format string, args ...any) {
	l.WithFields(l.fields).Infof(format, args...)
}

// Printf logs a message at level Info on the standard logger.
func (l *logger) Printf(format string, args ...any) {
	l.WithFields(l.fields).Printf(format, args...)
}

// Warnf logs a message at level Warn on the standard logger.
func (l *logger) Warnf(format string, args ...any) {
	l.WithFields(l.fields).Warnf(format, args...)
}

// Warningf logs a message at level Warn on the standard logger.
func (l *logger) Warningf(format string, args ...any) {
	l.WithFields(l.fields).Warningf(format, args...)
}

// Errorf logs a message at level Error on the standard logger.
func (l *logger) Errorf(format string, args ...any) {
	l.WithFields(l.fields).Errorf(format, args...)
}

// Debug logs a message at level Debug on the standard logger.
func (l *logger) Debug(args ...any) {
	l.WithFields(l.fields).Debug(args...)
}

// Info logs a message at level Info on the standard logger.
func (l *logger) Info(args ...any) {
	l.WithFields(l.fields).Info(args...)
}

// Warn logs a message at level Warn on the standard logger.
func (l *logger) Warn(args ...any) {
	l.WithFields(l.fields).Warn(args...)
}

// Warning logs a message at level Warn on the standard logger.
func (l *logger) Warning(args ...any) {
	l.WithFields(l.fields).Warning(args...)
}

// Error logs a message at level Error on the standard logger.
func (l *logger) Error(args ...any) {
	l.WithFields(l.fields).Error(args...)
}

// Debugln logs a message at level Debug on the standard logger.
func (l *logger) Debugln(args ...any) {
	l.WithFields(l.fields).Debugln(args...)
}

// Infoln logs a message at level Info on the standard logger.
func (l *logger) Infoln(args ...any) {
	l.WithFields(l.fields).Infoln(args...)
}

// Warnln logs a message at level Warn on the standard logger.
func (l *logger) Warnln(args ...any) {
	l.WithFields(l.fields).Warnln(args...)
}

// Warningln logs a message at level Warn on the standard logger.
func (l *logger) Warningln(args ...any) {
	l.WithFields(l.fields).Warningln(args...)
}

// Errorln logs a message at level Error on the standard logger.
func (l *logger) Errorln(args ...any) {
	l.WithFields(l.fields).Errorln(args...)
}

// Subscribe adds a field base on current logger and returns a new logger.
func (l *logger) Subscribe(key, val string) Interface {
	fields := make(map[string]any, len(l.fields)+1)
	for k, v := range l.fields {
		fields[k] = v
	}
	fields[key] = val
	return &logger{
		Logger: l.Logger,
		fields: fields,
	}
}

// InitLogger initializes the global logger.
func InitLogger(level logrus.Level) {
	l := &logrus.Logger{
		Out:          os.Stderr,
		Formatter:    new(logrus.TextFormatter),
		Hooks:        make(logrus.LevelHooks),
		Level:        logrus.InfoLevel,
		ExitFunc:     os.Exit,
		ReportCaller: false,
	}
	l.SetLevel(level)
	Logger = &logger{
		Logger: l,
		fields: map[string]any{},
	}
}
