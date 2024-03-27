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

package wsus

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

var (
	stoppedService     = "Wecsvc"          // This service should not be running by default.
	runningService     = "w32time"         // This service should always be running by default.
	testStoppedService = "TestService"     // A test service that will get created.
	badService         = "veryfakeservice" // This service should not exist.
)

func TestSortedKeys(t *testing.T) {
	for _, tt := range []struct {
		in  map[int]string
		out []int
	}{
		{map[int]string{2: "foo", 1: "bar", 3: "baz"}, []int{1, 2, 3}},
		{map[int]string{1: "foo", 2: "bar", 3: "baz"}, []int{1, 2, 3}},
	} {
		o := sortedKeys(tt.in)
		if !reflect.DeepEqual(o, tt.out) {
			t.Errorf("SortedKeys(%v) = %v, want %v", tt.in, o, tt.out)
		}
	}
}

func TestSet(t *testing.T) {
	clnUpdateFolder = func(name string) error { return nil }
	w := WSUS{
		CurrentServer:   "",
		ServerSelection: 0,
		Servers:         []string{"foo.biz", "bar.biz"},
	}

	for _, tt := range []struct {
		in            int
		currentServer string
		isNil         bool
	}{
		{2, "", false},
		{1, "bar.biz", true},
		{0, "foo.biz", true},
	} {
		err := w.Set(tt.in)
		if (err == nil) != tt.isNil {
			t.Errorf("Verify Nil Error:\nSet(%v) = %v, want %v", tt.in, err, tt.isNil)
		}
		if w.CurrentServer != tt.currentServer {
			t.Errorf("Verify Current Server:\nSet(%v) = %v, want %v", tt.in, w.CurrentServer, tt.currentServer)
		}
	}
}

func TestCleanUpdateFolder(t *testing.T) {
	// This is a directory that shouldn't exist.
	badDir := os.Getenv("windir") + `\rollinginthedeep`
	// The actual expected directory that will be cleared.
	updateDir := os.Getenv("windir") + `\SoftwareDistribution`
	for _, tt := range []struct {
		dir     string
		wantErr bool
	}{
		{badDir, true},
		{updateDir, false},
	} {
		if err := cleanUpdateFolder(tt.dir); (err != nil) != tt.wantErr {
			t.Errorf("cleanUpdateFolder(%s) = %v, want error presence = %v", tt.dir, err, tt.wantErr)
		}
	}
}

func TestCleanUpdateFolderWithOpenFile(t *testing.T) {
	strtService = func(name string) error { return nil }
	stpService = func(name string) error { return nil }
	dir, err := os.MkdirTemp("", "setfiretotherain")
	if err != nil {
		t.Fatalf("TestCleanUpdateFolderWithOpenFile setup failed: could not make temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	f, err := os.CreateTemp(dir, "lyrics")
	if err != nil {
		t.Fatalf("TestCleanUpdateFolderWithOpenFile setup failed: could not create temp file in %s: %v", dir, err)
	}

	// Hold the file open so it can't be deleted.
	d, err := os.Open(f.Name())
	if err != nil {
		t.Fatalf("TestCleanUpdateFolderWithOpenFile setup failed: could not open temp file(%s): %v", f.Name(), err)
	}
	if err := cleanUpdateFolder(dir); err == nil {
		t.Errorf("cleanUpdateFolder(%v) returned nil, want error: %v", dir, err)
	}
	d.Close()
}

func TestStartService(t *testing.T) {
	for _, tt := range []struct {
		service string
		wantErr bool
	}{
		{stoppedService, false},
		{runningService, false},
		{badService, true},
	} {
		if err := startService(tt.service); (err != nil) != tt.wantErr {
			t.Errorf("StartService(%s) = %v, want error presence = %v", tt.service, err, tt.wantErr)
		}
	}
}

func createStoppedServiceHelper(t *testing.T) {
	t.Helper()
	cmdPath := `C:\Windows\System32\cmd.exe`
	pwshPath := `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
	serviceParams := fmt.Sprintf("New-Service -Name %s -BinaryPathName '%s'", testStoppedService, cmdPath)
	cmd := exec.Command(pwshPath, serviceParams)
	if _, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("createStoppedServiceHelper setup failed: could not create test service: %v", err)
	}
}

func TestStopService(t *testing.T) {
	createStoppedServiceHelper(t)
	for _, tt := range []struct {
		service string
		wantErr bool
	}{
		{runningService, false},
		{testStoppedService, false},
		{badService, true},
	} {
		err := stopService(tt.service)
		gotErr := err != nil
		if gotErr != tt.wantErr {
			t.Errorf("StopService(%s) = %v, want error presence = %v", tt.service, err, tt.wantErr)
		}
	}
}
