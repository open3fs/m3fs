package base

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"

	"github.com/bitly/go-simplejson"
	guuid "github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/errors"
)

// Suite is the base Suite for all test suites.
type Suite struct {
	suite.Suite
}

// SetupSuite runs before all tests in the suite.
func (s *Suite) SetupSuite() {
}

// TearDownSuite runs after all tests in the suite.
func (s *Suite) TearDownSuite() {
}

// SetupTest runs before each test in the suite.
func (s *Suite) SetupTest() {
}

// TearDownTest runs after each test in the suite.
func (s *Suite) TearDownTest() {
}

// Log output to error log.
func (s *Suite) Log(a ...any) {
	s.T().Log(a...)
}

// Logf output to error log.
func (s *Suite) Logf(format string, a ...any) {
	s.T().Logf(format, a...)
}

// PrettyDump prints Golang objects in a beautiful way.
func (s *Suite) PrettyDump(a ...any) {
	common.PrettyDump(a...)
}

// PrettySdump prints Golang objects in a beautiful way to string.
func PrettySdump(a ...any) string {
	return common.PrettySdump(a...)
}

// R returns a require context.
func (s *Suite) R() *require.Assertions {
	return s.Require()
}

// NoError require no error
func (s *Suite) NoError(err error, args ...any) {
	if err != nil {
		err = fmt.Errorf("%s", errors.StackTrace(err))
	}
	s.R().NoError(err, args...)
}

// InDelta require within delta.
func (s *Suite) InDelta(e, a any, delta float64, msg ...any) {
	s.R().InDelta(e, a, delta, msg...)
}

// Equal require equal
func (s *Suite) Equal(e, a any, msg ...any) {
	s.R().Equal(e, a, msg...)
}

// Empty require empty
func (s *Suite) Empty(object any, msg ...any) {
	s.R().Empty(object, msg...)
}

// Len require len
func (s *Suite) Len(object any, l int, args ...any) {
	s.R().Len(object, l, args...)
}

// Nil require nil
func (s *Suite) Nil(object any, args ...any) {
	s.R().Nil(object, args...)
}

// NotNil require not nil
func (s *Suite) NotNil(object any, args ...any) {
	s.R().NotNil(object, args...)
}

// True require true
func (s *Suite) True(value bool, args ...any) {
	s.R().True(value, args...)
}

// False require false
func (s *Suite) False(value bool, args ...any) {
	s.R().False(value, args...)
}

// Zero require 0
func (s *Suite) Zero(i any, args ...any) {
	s.R().Zero(i, args...)
}

// NotZero require not zero
func (s *Suite) NotZero(i any, args ...any) {
	s.R().NotZero(i, args...)
}

// Contains require that the specified string, list(array, slice...) or map contains the specified
// substring or element.
func (s *Suite) Contains(object, contains any, args ...any) {
	s.R().Contains(object, contains, args...)
}

// AssertNilEqual asserts two objects are both nil or non-nil.
// Return true if they are both non-nil, false for both ni..
func (s *Suite) AssertNilEqual(e, a any, msg ...any) bool {
	eValue := reflect.ValueOf(e)
	aValue := reflect.ValueOf(a)
	if !eValue.IsNil() && !aValue.IsNil() {
		return true
	}
	if eValue.IsNil() && !aValue.IsNil() {
		if len(msg) > 0 {
			s.R().Fail(fmt.Sprintf("expected value is nil while actual not: %s", msg...))
		} else {
			s.R().Fail("expected value is nil while actual not")
		}
	}
	if !eValue.IsNil() && aValue.IsNil() {
		if len(msg) > 0 {
			s.R().Fail(fmt.Sprintf("actual value is nil while expected not: %s", msg...))
		} else {
			s.R().Fail("actual value is nil while expected not")
		}
	}
	return false
}

// Ctx returns a context used in test.
func (s *Suite) Ctx() context.Context {
	return context.TODO()
}

// RandInt63 returns a non-negative pseudo-random 63-bit integer as an int64.
func (s *Suite) RandInt63() int64 {
	return rand.Int63()
}

// NewError create s a new error.
func (s *Suite) NewError(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

// NewUUID creates a new UUID.
func (s *Suite) NewUUID() string {
	uuid, _ := guuid.NewRandom()
	return uuid.String()
}

// NewEtag creates a new Etag.
func (s *Suite) NewEtag() string {
	return s.NewUUID()
}

// NewEnvID returns an random env id for testing.
func (s *Suite) NewEnvID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, rand.Int63())
}

// NewMacAddress returns a random mac address.
func (s *Suite) NewMacAddress() string {
	value := s.RandInt63() & 0x0000ffffffffffff
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		value>>40&0xff,
		value>>32&0xff,
		value>>24&0xff,
		value>>16&0xff,
		value>>8&0xff,
		value&0xff,
	)
}

// NewNodeGUID returns a random guid
func (s *Suite) NewNodeGUID() string {
	value := s.RandInt63() & 0x0000ffffffffffff
	return fmt.Sprintf("%02x:%02x:%02x:fe:ff:%02x:%02x:%02x",
		value>>40&0xff,
		value>>32&0xff,
		value>>24&0xff,
		value>>16&0xff,
		value>>8&0xff,
		value&0xff,
	)
}

// NewPciAddress returns a random pci address.
func (s *Suite) NewPciAddress() string {
	value := s.RandInt63() & 0x0000000fffffffff
	return fmt.Sprintf("%04x:%02x:%02x.%01x",
		value>>20&0xffff,
		value>>12&0xff,
		value>>4&0xff,
		value&0xf,
	)
}

// JsonToString convert json to string
func (s *Suite) JsonToString(dataJSON *simplejson.Json) string {
	data, err := dataJSON.MarshalJSON()
	s.NoError(err)
	return string(data)
}

// JsonMarshal marshal object to json.
func (s *Suite) JsonMarshal(v any) []byte {
	out, err := json.Marshal(v)
	s.NoError(err)
	return out
}

// JsonUnmarshal unmarshal json doc to object.
func (s *Suite) JsonUnmarshal(data []byte, dest any) {
	s.NoError(json.Unmarshal(data, dest))
}

// YamlMarshal marshal object to yaml.
func (s *Suite) YamlMarshal(v any) []byte {
	out, err := yaml.Marshal(v)
	s.NoError(err)
	return out
}

// YamlUnmarshal unmarshal json doc to object.
func (s *Suite) YamlUnmarshal(data []byte, dest any) {
	s.NoError(yaml.Unmarshal(data, dest))
}

// CopyFields copy fields between struct.
func (s *Suite) CopyFields(dest, src any, fields ...string) {
	s.NoError(common.CopyFields(dest, src, fields...))
}

// Sprint wrap fmt.Sprint
func (s *Suite) Sprint(a ...any) string {
	return fmt.Sprint(a...)
}

// Sprintf wrap fmt.Sprintf
func (s *Suite) Sprintf(fmtStr string, a ...any) string {
	return fmt.Sprintf(fmtStr, a...)
}
