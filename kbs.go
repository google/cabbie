// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"fmt"
	"strings"

	"github.com/google/glazier/go/helpers"
)

// KBSet models a group of update KBs
type KBSet struct {
	kbSlice []string
	kbMap   map[string]bool
}

// NewKBSet creates a new KBSet given a comma-separated list of KB article IDs
func NewKBSet(kbList string) KBSet {
	return NewKBSetFromSlice(helpers.StringToSlice(kbList))
}

// NewKBSetFromSlice creates a new KBSet given a slice of KB article IDs
func NewKBSetFromSlice(kbSlice []string) KBSet {
	kbMap := make(map[string]bool)
	for _, kb := range kbSlice {
		kb = strings.ReplaceAll(strings.ToLower(kb), "kb", "")
		kbMap[kb] = true
	}
	return KBSet{
		kbSlice: kbSlice,
		kbMap:   kbMap,
	}
}

// Search searches the KBSet for a list of identifiers and returns true if any match.
func (u KBSet) Search(ids []string) bool {
	for _, v := range ids {
		v = strings.ReplaceAll(strings.ToLower(v), "kb", "")
		if u.kbMap[v] {
			return true
		}
	}
	return false
}

// Size returns the size of the set (number of updates).
func (u KBSet) Size() int {
	return len(u.kbSlice)
}

// String renders the KBSet as a string.
func (u KBSet) String() string {
	return fmt.Sprintf("%v", u.kbSlice)
}
