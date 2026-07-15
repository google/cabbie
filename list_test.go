// Copyright 2026 Google LLC
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

//go:build windows
// +build windows

package main

import (
	"testing"

	"flag"
)

func TestListCmdMetadata(t *testing.T) {
	cmd := &listCmd{}

	if got := cmd.Name(); got != "list" {
		t.Errorf("Name() = %q, want %q", got, "list")
	}
	if got := cmd.Synopsis(); got == "" {
		t.Errorf("Synopsis() is empty")
	}
	if got := cmd.Usage(); got == "" {
		t.Errorf("Usage() is empty")
	}
}

func TestListCmdSetFlags(t *testing.T) {
	cmd := &listCmd{}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	cmd.SetFlags(fs)

	if f := fs.Lookup("hidden"); f == nil {
		t.Errorf("flag 'hidden' not found")
	}
	if f := fs.Lookup("ids"); f == nil {
		t.Errorf("flag 'ids' not found")
	}
}
