// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/google/cabbie/enforcement"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/updates"
	"github.com/google/go-cmp/cmp"
)

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

func TestNewUpdateRunner(t *testing.T) {
	opts := UpdateOptions{
		RequiredCategories: []string{"Security Updates"},
	}
	cmdNormal := &installCmd{virusDef: false}
	runnerNormal := NewUpdateRunner(nil, nil, cmdNormal, opts)
	if runnerNormal.installMsgPopped {
		t.Errorf("NewUpdateRunner with virusDef=false: installMsgPopped got true, want false")
	}

	cmdVirus := &installCmd{virusDef: true}
	runnerVirus := NewUpdateRunner(nil, nil, cmdVirus, opts)
	if !runnerVirus.installMsgPopped {
		t.Errorf("NewUpdateRunner with virusDef=true: installMsgPopped got false, want true")
	}
}

func TestUpdateRunner_ShouldSkip(t *testing.T) {
	cmd := &installCmd{}
	opts := UpdateOptions{
		RequiredCategories: []string{"Security Updates"},
		DriverExcludes: []enforcement.DriverExclude{
			{DriverClass: "Net"},
		},
	}
	runner := NewUpdateRunner(nil, nil, cmd, opts)

	tests := []struct {
		name string
		up   *updates.Update
		want bool
	}{
		{
			name: "DriverExcluded",
			up: &updates.Update{
				Title:       "Test Net Driver",
				DriverClass: "Net",
				Categories:  []updates.Category{{Name: "Drivers"}},
			},
			want: true,
		},
		{
			name: "CategoryMismatch",
			up: &updates.Update{
				Title:      "Test Optional Update",
				Categories: []updates.Category{{Name: "Tools"}},
			},
			want: true,
		},
		{
			name: "CategoryMatch",
			up: &updates.Update{
				Title:      "Test Security Update",
				Categories: []updates.Category{{Name: "Security Updates"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runner.shouldSkip(tt.up)
			if got != tt.want {
				t.Errorf("shouldSkip(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestUpdateRunner_RecordRebootState(t *testing.T) {
	opts := UpdateOptions{}
	runner := NewUpdateRunner(nil, nil, nil, opts)

	secUpdate := &updates.Update{
		Title:        "Security KB12345",
		KBArticleIDs: []string{"12345"},
		Categories:   []updates.Category{{Name: "Security Updates"}},
	}

	rspReboot := &installRsp{
		resultCode:     2,
		rebootRequired: true,
	}

	runner.recordRebootState(secUpdate, rspReboot)
	if !runner.anyReboot {
		t.Errorf("recordRebootState for Security Update with rebootRequired=true: anyReboot got false, want true")
	}
	if diff := cmp.Diff([]string{"12345"}, runner.rebootList); diff != "" {
		t.Errorf("recordRebootState rebootList diff (-want +got):\n%s", diff)
	}

	// Definition updates requiring reboot should NOT trigger host reboot tracking
	runnerDef := NewUpdateRunner(nil, nil, nil, opts)
	defUpdate := &updates.Update{
		Title:        "Def KB99999",
		KBArticleIDs: []string{"99999"},
		Categories:   []updates.Category{{Name: "Definition Updates"}},
	}
	runnerDef.recordRebootState(defUpdate, rspReboot)
	if runnerDef.anyReboot {
		t.Errorf("recordRebootState for Definition Update with rebootRequired=true: anyReboot got true, want false")
	}
}

func TestUpdateRunner_ShouldSkip_NilUpdate(t *testing.T) {
	opts := UpdateOptions{
		DriverExcludes: []enforcement.DriverExclude{
			{DriverClass: "Net"},
		},
	}
	runner := NewUpdateRunner(nil, nil, nil, opts)

	if got := runner.shouldSkip(nil); !got {
		t.Errorf("shouldSkip(nil) = %v, want true", got)
	}
}

func TestUpdateRunner_EnsureEulaAccepted(t *testing.T) {
	opts := UpdateOptions{}
	runner := NewUpdateRunner(nil, nil, nil, opts)

	// 1. EULA already accepted -> no-op
	acceptedUp := &updates.Update{
		Title:        "Accepted Update",
		EulaAccepted: true,
	}
	runner.ensureEulaAccepted(acceptedUp)
	if !acceptedUp.EulaAccepted {
		t.Errorf("ensureEulaAccepted on accepted update modified EulaAccepted")
	}

	// 2. EULA unaccepted, nil Item -> calls AcceptEula() which returns error ("Update.Item is nil"), handled gracefully without panic
	unacceptedUp := &updates.Update{
		Title:        "Unaccepted Update with nil Item",
		EulaAccepted: false,
		Item:         nil,
	}
	runner.ensureEulaAccepted(unacceptedUp)
	if unacceptedUp.EulaAccepted {
		t.Errorf("ensureEulaAccepted on nil Item update unexpectedly set EulaAccepted to true")
	}

	// 3. nil update -> handled gracefully without panic
	runner.ensureEulaAccepted(nil)
}

func TestUpdateRunner_TriggerPreUpdateHooks(t *testing.T) {
	opts := UpdateOptions{}

	// Non-definition update should pop install msg and set installedAny
	runner1 := NewUpdateRunner(nil, nil, nil, opts)
	secUp := &updates.Update{
		Title:      "Security Update",
		Categories: []updates.Category{{Name: "Security Updates"}},
	}
	runner1.triggerPreUpdateHooks(secUp)
	if !runner1.installMsgPopped {
		t.Errorf("triggerPreUpdateHooks for Security Update: installMsgPopped got false, want true")
	}
	if !runner1.installedAny {
		t.Errorf("triggerPreUpdateHooks for Security Update: installedAny got false, want true")
	}

	// Definition update should NOT pop install msg
	runner2 := NewUpdateRunner(nil, nil, nil, opts)
	defUp := &updates.Update{
		Title:      "Def Update",
		Categories: []updates.Category{{Name: "Definition Updates"}},
	}
	runner2.triggerPreUpdateHooks(defUp)
	if runner2.installMsgPopped {
		t.Errorf("triggerPreUpdateHooks for Definition Update: installMsgPopped got true, want false")
	}
	if runner2.installedAny {
		t.Errorf("triggerPreUpdateHooks for Definition Update: installedAny got true, want false")
	}
}

func TestUpdateRunner_DownloadSingleUpdate_NilCollection(t *testing.T) {
	opts := UpdateOptions{}
	runner := NewUpdateRunner(nil, nil, nil, opts)

	up := &updates.Update{Title: "Test Update"}

	if err := runner.downloadSingleUpdate(up, nil); err == nil {
		t.Errorf("downloadSingleUpdate(up, nil) expected error, got nil")
	}
}

func TestUpdateRunner_InstallSingleUpdate_NilCollection(t *testing.T) {
	opts := UpdateOptions{}
	runner := NewUpdateRunner(nil, nil, nil, opts)

	up := &updates.Update{Title: "Test Update"}

	if _, err := runner.installSingleUpdate(up, nil); err == nil {
		t.Errorf("installSingleUpdate(up, nil) expected error, got nil")
	}
}

func TestUpdateRunner_ProcessSingleUpdate_Skipped(t *testing.T) {
	cmd := &installCmd{}
	opts := UpdateOptions{
		RequiredCategories: []string{"Security Updates"},
	}
	runner := NewUpdateRunner(nil, nil, cmd, opts)

	// Update that should be skipped (Tools category when only Security Updates required)
	up := &updates.Update{
		Title:      "Optional Tool",
		Categories: []updates.Category{{Name: "Tools"}},
	}

	res := runner.ProcessSingleUpdate(up)
	if !res.Skipped {
		t.Errorf("ProcessSingleUpdate for skipped update: Skipped got false, want true")
	}
	if res.Err != nil {
		t.Errorf("ProcessSingleUpdate for skipped update: Err got %v, want nil", res.Err)
	}
}

func TestUpdateRunner_ProcessSingleUpdate_EulaFailure_DownloadFailure(t *testing.T) {
	cmd := &installCmd{}
	opts := UpdateOptions{
		RequiredCategories: []string{"Security Updates"},
	}
	runner := NewUpdateRunner(nil, nil, cmd, opts)

	// Update with unaccepted EULA and nil Item (will fail collection creation / COM download in test env)
	up := &updates.Update{
		Title:        "Unaccepted Security KB111",
		KBArticleIDs: []string{"111"},
		EulaAccepted: false,
		Categories:   []updates.Category{{Name: "Security Updates"}},
	}

	res := runner.ProcessSingleUpdate(up)
	if res.Skipped {
		t.Errorf("ProcessSingleUpdate for matching category: Skipped got true, want false")
	}
	if res.Err == nil {
		t.Errorf("ProcessSingleUpdate expected error when downloading without valid collection/session, got nil")
	}
}

func TestUpdateRunner_Finalize_NoOps(t *testing.T) {
	opts := UpdateOptions{}
	runner := NewUpdateRunner(nil, nil, nil, opts)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Finalize() panicked unexpectedly: %v", r)
		}
	}()
	runner.Finalize()
}



