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
	"strings"
	"testing"

	"github.com/google/cabbie/search"
	"github.com/google/go-cmp/cmp"
)

type testInstallLog struct {
}

func (f *testInstallLog) Info(id uint32, msg string) error {
	return nil
}
func (f *testInstallLog) Error(id uint32, msg string) error {
	return nil
}
func (f *testInstallLog) Warning(id uint32, msg string) error {
	return nil
}
func (f *testInstallLog) Close() error {
	return nil
}

func newFakeConfig() *Settings {
	return &Settings{RequiredCategories: categoryDefaults}
}

func TestName(t *testing.T) {
	name := "install"
	install := &installCmd{}
	got := install.Name()
	if got != name {
		t.Errorf("Name() got: %q, want: %q", got, name)
	}
}

func TestSynopsis(t *testing.T) {
	install := &installCmd{}
	got := install.Synopsis()
	if got == "" {
		t.Errorf("Synopsis() got: %q, want: not empty", got)
	}
}

func TestUsage(t *testing.T) {
	install := &installCmd{}
	got := install.Usage()
	if got == "" {
		t.Errorf("Usage() got: %q, want: not empty", got)
	}
}

func TestVetFlags(t *testing.T) {
	for _, tt := range []struct {
		cmd  installCmd
		want error
	}{
		{installCmd{}, nil},
		{installCmd{drivers: true}, nil},
		{installCmd{virusDef: true}, nil},
		{installCmd{kbs: "12345,54321"}, nil},
		{installCmd{drivers: true, kbs: "12345,54321"}, errInvalidFlags},
	} {
		got := vetFlags(tt.cmd)
		if got != tt.want {
			t.Errorf("vetFlags(%v): got %t, want %t", tt.cmd, got, tt.want)
		}
	}
}

func TestGetCriteria(t *testing.T) {
	for _, tt := range []struct {
		i              installCmd
		outcriteria    string
		outRequiredCat []string
	}{
		{installCmd{drivers: true}, "Driver", []string{"Drivers"}},
		{installCmd{virusDef: true}, string(search.DefinitionUpdates), []string{"Definition Updates"}},
		{installCmd{kbs: "KB1234567"}, string(search.BasicSearch), nil},
		{installCmd{}, string(search.BasicSearch), categoryDefaults},
	} {
		elog = new(testInstallLog)
		config = newFakeConfig()
		oc, orc := tt.i.criteria()
		if !(strings.Contains(oc, tt.outcriteria)) {
			t.Errorf("criteria test got %s, want %s", oc, tt.outcriteria)
		}
		if diff := cmp.Diff(tt.outRequiredCat, orc); diff != "" {
			t.Errorf("TestGetCriteria(%v) returned diff (-want +got):\n%s", tt.outRequiredCat, diff)
		}
	}
}
