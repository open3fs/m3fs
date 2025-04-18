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
	"strings"
	"sync"
)

// StringBuilderPool provides a reusable pool of string builders
var StringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// GetStringBuilder gets a strings.Builder from the object pool
func GetStringBuilder() *strings.Builder {
	sb := StringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// PutStringBuilder returns a strings.Builder to the object pool
func PutStringBuilder(sb *strings.Builder) {
	StringBuilderPool.Put(sb)
}

// SlicePool provides a generic slice pool
type SlicePool struct {
	pool    sync.Pool
	initCap int
}

// NewStringSlicePool creates a new SlicePool for string slices
func NewStringSlicePool(initialCapacity int) *SlicePool {
	return &SlicePool{
		pool: sync.Pool{
			New: func() any {
				return make([]string, 0, initialCapacity)
			},
		},
		initCap: initialCapacity,
	}
}

// GetStringSlice gets a string slice from the pool
func (p *SlicePool) GetStringSlice() []string {
	if v := p.pool.Get(); v != nil {
		return v.([]string)[:0]
	}
	return make([]string, 0, p.initCap)
}

// Put returns a slice to the pool
func (p *SlicePool) Put(slice any) {
	p.pool.Put(slice)
}

// MapPool provides a generic map pool
type MapPool struct {
	pool    sync.Pool
	initCap int
}

// NewStringMapPool creates a new MapPool for string maps
func NewStringMapPool(initialCapacity int) *MapPool {
	return &MapPool{
		pool: sync.Pool{
			New: func() any {
				return make(map[string]struct{}, initialCapacity)
			},
		},
		initCap: initialCapacity,
	}
}

// GetStringMap gets a map from the pool and clears it
func (p *MapPool) GetStringMap() map[string]struct{} {
	if v := p.pool.Get(); v != nil {
		m := v.(map[string]struct{})
		for k := range m {
			delete(m, k)
		}
		return m
	}
	return make(map[string]struct{}, p.initCap)
}

// Put returns a map to the pool
func (p *MapPool) Put(m any) {
	p.pool.Put(m)
}
