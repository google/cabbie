// Copyright 2024 Google LLC
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
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/google/cabbie/cablib"
	"golang.org/x/sys/windows/registry"
)

const testRegPath = `SOFTWARE\Cabbie_test`

type mockConn struct {
	net.Conn
}

func (m *mockConn) Close() error {
	return nil
}
func (m *mockConn) Read(b []byte) (n int, err error) {
	return 0, nil
}
func (m *mockConn) Write(b []byte) (n int, err error) {
	return 0, nil
}
func (m *mockConn) RemoteAddr() net.Addr {
	return nil
}
func (m *mockConn) LocalAddr() net.Addr {
	return nil
}
func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}
func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestSetWsusIfNeeded(t *testing.T) {
	tests := []struct {
		name       string
		dialErr    error
		force      bool
		targets    string
		wantErr    bool
		wantSet    bool
		wantValues []string
	}{
		{
			name:    "NoDialError_NoForce",
			dialErr: nil,
			force:   false,
			targets: "server1,server2",
			wantErr: false,
			wantSet: false,
		},
		{
			name:       "DialError_NoForce",
			dialErr:    errors.New("dial error"),
			force:      false,
			targets:    "server1,server2",
			wantErr:    false,
			wantSet:    true,
			wantValues: []string{"server1", "server2"},
		},
		{
			name:       "NoDialError_Force",
			dialErr:    nil,
			force:      true,
			targets:    "server1,server2",
			wantErr:    false,
			wantSet:    true,
			wantValues: []string{"server1", "server2"},
		},
		{
			name:    "EmptyTargets",
			dialErr: errors.New("dial error"),
			force:   false,
			targets: "",
			wantErr: true,
			wantSet: false,
		},
		{
			name:    "EmptyTargetsWithComma",
			dialErr: errors.New("dial error"),
			force:   false,
			targets: ",",
			wantErr: true,
			wantSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use test registry path
			origPath := cablib.RegPath
			cablib.RegPath = testRegPath
			// Clean registry before and after test
			registry.DeleteKey(registry.LOCAL_MACHINE, testRegPath)
			defer func() {
				cablib.RegPath = origPath
				registry.DeleteKey(registry.LOCAL_MACHINE, testRegPath)
			}()

			netDialTimeout = func(network, address string, timeout time.Duration) (net.Conn, error) {
				if tt.dialErr != nil {
					return nil, tt.dialErr
				}
				return &mockConn{}, nil
			}
			err := setWsusIfNeeded(tt.targets, tt.force)
			if (err != nil) != tt.wantErr {
				t.Errorf("setWsusIfNeeded() error = %v, wantErr %v", err, tt.wantErr)
			}
			k, err := registry.OpenKey(registry.LOCAL_MACHINE, testRegPath, registry.QUERY_VALUE)
			if err == registry.ErrNotExist {
				if !tt.wantSet {
					return
				}
				t.Fatalf("Registry key %s not found, but WSUSServers value was expected to be set.", testRegPath)
			}
			if err != nil {
				t.Fatalf("Failed to open registry key %s: %v", testRegPath, err)
			}
			defer k.Close()

			gotValues, _, err := k.GetStringsValue("WSUSServers")
			gotSet := err == nil
			if gotSet != tt.wantSet {
				t.Errorf("WSUSServers registry value set = %v, wantSet %v", gotSet, tt.wantSet)
			}
			if tt.wantSet && !reflect.DeepEqual(gotValues, tt.wantValues) {
				t.Errorf("WSUSServers registry value = %v, wantValues %v", gotValues, tt.wantValues)
			}
		})
	}
}
