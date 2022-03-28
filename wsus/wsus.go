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

// Package wsus configures the local update client with the fastest server.
package wsus

import (
	"sort"
)

const (
	// Default indicates that the search call should search the default server.
	// If the computer is not been set up to have a managed server (WSUS),
	// WUA uses the first update service which the IsRegisteredWithAU property is true.
	Default = iota
	// ManagedServer indicates to use the configured WSUS server.
	ManagedServer
	// WindowsUpdate indicates the Microsoft Windows Update service.
	WindowsUpdate
	// Others indicates some update service other than those listed previously.
	// if selected, ServiceID must be set to a registered ID.
	Others
)

// WSUS contains local managed server information.
type WSUS struct {
	CurrentServer   string
	ServerSelection int
	Servers         []string
}

func sortedKeys(s map[int]string) []int {
	var keys []int
	for k := range s {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
