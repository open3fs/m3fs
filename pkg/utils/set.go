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

package utils

import (
	"github.com/open3fs/m3fs/pkg/common"
)

// Set is a set
type Set[T comparable] map[T]struct{}

// Add adds an item to the set
func (s Set[T]) Add(item T) {
	s[item] = struct{}{}
}

// Remove removes an item from the set
func (s Set[T]) Remove(item T) {
	delete(s, item)
}

// Contains returns true if the set contains the item
func (s Set[T]) Contains(item T) bool {
	_, ok := s[item]
	return ok
}

// Len returns the number of items in the set
func (s Set[T]) Len() int {
	return len(s)
}

// AddIfNotExists adds an item to the set if it does not already exist
func (s Set[T]) AddIfNotExists(item T) bool {
	add := false
	if !s.Contains(item) {
		s.Add(item)
		add = true
	}

	return add
}

// ToSlice converts the set to a slice
func (s Set[T]) ToSlice() []T {
	ret := make([]T, 0, len(s))
	for item := range s {
		ret = append(ret, item)
	}
	return ret
}

// Equal check two set equal
func (s Set[T]) Equal(other Set[T]) bool {
	if len(s) != len(other) {
		return false
	}
	for item := range s {
		if _, ok := other[item]; !ok {
			return false
		}
	}
	return true
}

// Difference returns a new set containing elements that exists in set a but not in set b
func (s Set[T]) Difference(other Set[T]) Set[T] {
	result := make(Set[T])
	for item := range s {
		if !other.Contains(item) {
			result.Add(item)
		}
	}

	return result
}

// NewSet creates a new Set
func NewSet[T comparable](elems ...T) *Set[T] {
	s := common.Pointer(make(Set[T]))
	for _, elem := range elems {
		s.Add(elem)
	}
	return s
}
