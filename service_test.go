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

func TestServiceCmdMetadata(t *testing.T) {
	cmd := &serviceCmd{}

	if got := cmd.Name(); got != "service" {
		t.Errorf("Name() = %q, want %q", got, "service")
	}
	if got := cmd.Synopsis(); got == "" {
		t.Errorf("Synopsis() is empty")
	}
	if got := cmd.Usage(); got == "" {
		t.Errorf("Usage() is empty")
	}
}

func TestServiceCmdSetFlags(t *testing.T) {
	cmd := &serviceCmd{}
	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	cmd.SetFlags(fs)

	if f := fs.Lookup("install"); f == nil {
		t.Errorf("flag 'install' not found")
	}
	if f := fs.Lookup("uninstall"); f == nil {
		t.Errorf("flag 'uninstall' not found")
	}
}

func TestServiceCmdExecuteConflictingFlags(t *testing.T) {
	cmd := &serviceCmd{install: true, uninstall: true}
	fs := flag.NewFlagSet("service", flag.ContinueOnError)

	ctx := context.Background()
	status := cmd.Execute(ctx, fs)
	if status != subcommands.ExitFailure {
		t.Errorf("Execute(install=true, uninstall=true) = %v, want ExitFailure (%v)", status, subcommands.ExitFailure)
	}
}

func TestServiceCmdExecuteNoFlags(t *testing.T) {
	cmd := &serviceCmd{install: false, uninstall: false}
	fs := flag.NewFlagSet("service", flag.ContinueOnError)

	ctx := context.Background()
	status := cmd.Execute(ctx, fs)
	if status != subcommands.ExitUsageError {
		t.Errorf("Execute(install=false, uninstall=false) = %v, want ExitUsageError (%v)", status, subcommands.ExitUsageError)
	}
}
