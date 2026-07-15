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

func TestRebootCmdMetadata(t *testing.T) {
	cmd := &rebootCmd{}

	if got := cmd.Name(); got != "reboot" {
		t.Errorf("Name() = %q, want %q", got, "reboot")
	}
	if got := cmd.Synopsis(); got == "" {
		t.Errorf("Synopsis() is empty")
	}
	if got := cmd.Usage(); got == "" {
		t.Errorf("Usage() is empty")
	}
}

func TestRebootCmdSetFlags(t *testing.T) {
	cmd := &rebootCmd{}
	fs := flag.NewFlagSet("reboot", flag.ContinueOnError)
	cmd.SetFlags(fs)

	if f := fs.Lookup("clear"); f == nil {
		t.Errorf("flag 'clear' not found")
	}
	if f := fs.Lookup("time"); f == nil {
		t.Errorf("flag 'time' not found")
	}
	if f := fs.Lookup("check"); f == nil {
		t.Errorf("flag 'check' not found")
	}
}

func TestRebootCmdExecuteNoFlags(t *testing.T) {
	cmd := &rebootCmd{}
	fs := flag.NewFlagSet("reboot", flag.ContinueOnError)
	cmd.SetFlags(fs)

	ctx := context.Background()
	status := cmd.Execute(ctx, fs)
	if status != subcommands.ExitFailure {
		t.Errorf("Execute() with no flags = %v, want ExitFailure (%v)", status, subcommands.ExitFailure)
	}
}
