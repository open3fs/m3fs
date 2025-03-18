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

package common

import (
	"math/rand"
	"time"
)

var (
	chars = [...]rune{
		'a', 'b', 'c', 'd', 'e', 'f', 'g',
		'h', 'i', 'j', 'k', 'l', 'm', 'n',
		'o', 'p', 'q', 'r', 's', 't',
		'u', 'v', 'w', 'x', 'y', 'z',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	}
)

// RandomString return random string with specified length
func RandomString(strlen int, withoutNumber ...bool) string {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	result := make([]rune, strlen)
	var max int
	if len(withoutNumber) > 0 && withoutNumber[0] {
		max = len(chars) - 10
	} else {
		max = len(chars)
	}
	for i := 0; i < strlen; i++ {
		result[i] = chars[rand.Intn(max)]
	}
	return string(result)
}
