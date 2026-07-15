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
	"golang.org/x/net/context"
	"testing"

	"flag"
	"github.com/google/subcommands"
)

func TestHideCmdMetadata(t *testing.T) {
	cmd := &hideCmd{}

	if got := cmd.Name(); got != "hide" {
		t.Errorf("Name() = %q, want %q", got, "hide")
	}
	if got := cmd.Synopsis(); got == "" {
		t.Errorf("Synopsis() is empty")
	}
	if got := cmd.Usage(); got == "" {
		t.Errorf("Usage() is empty")
	}
}

func TestHideCmdSetFlags(t *testing.T) {
	cmd := &hideCmd{}
	fs := flag.NewFlagSet("hide", flag.ContinueOnError)
	cmd.SetFlags(fs)

	if f := fs.Lookup("kbs"); f == nil {
		t.Errorf("flag 'kbs' not found")
	}
	if f := fs.Lookup("unhide"); f == nil {
		t.Errorf("flag 'unhide' not found")
	}
}

func TestHideCmdExecuteNoKBs(t *testing.T) {
	cmd := &hideCmd{}
	fs := flag.NewFlagSet("hide", flag.ContinueOnError)
	cmd.SetFlags(fs)

	ctx := context.Background()
	status := cmd.Execute(ctx, fs)
	if status != subcommands.ExitUsageError {
		t.Errorf("Execute() with no KBs = %v, want ExitUsageError (%v)", status, subcommands.ExitUsageError)
	}
}
