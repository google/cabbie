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
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/windows/registry"
)

const (
	testPath = `SOFTWARE\Bar`
)

type testCabbieLog struct {
}

func (f *testCabbieLog) Info(id uint32, msg string) error {
	return nil
}
func (f *testCabbieLog) Error(id uint32, msg string) error {
	return nil
}
func (f *testCabbieLog) Warning(id uint32, msg string) error {
	return nil
}
func (f *testCabbieLog) Close() error {
	return nil
}

func createTestKeys() error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, testPath, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	return k.Close()
}

func cleanupTestKey() error {
	return registry.DeleteKey(registry.LOCAL_MACHINE, testPath)
}

func TestRegLoadKeyMissing(t *testing.T) {
	// Setup
	elog = new(testCabbieLog)
	expected := newSettings()
	testconfig := newSettings()
	// End Setup
	if err := testconfig.regLoad(testPath); err != registry.ErrNotExist {
		t.Error(err)
	}
	if !(cmp.Equal(testconfig, expected)) {
		t.Errorf("testconfig.regload(%s) = %v, want %v", testPath, testconfig, expected)
	}
}

func TestRegLoadKeyEmpty(t *testing.T) {
	// Setup
	elog = new(testCabbieLog)
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	expected := newSettings()
	testconfig := newSettings()
	// End Setup
	if err := testconfig.regLoad(testPath); err != nil {
		t.Error(err)
	}
	if !(cmp.Equal(testconfig, expected)) {
		t.Errorf("testconfig.regLoad(%s) = %v, want %v", testPath, testconfig, expected)
	}
}

func TestRegLoadRequiredCategories(t *testing.T) {
	// Setup
	rc := []string{"Bar", "Foo"}
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, testPath, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.SetStringsValue("RequiredCategories", rc); err != nil {
		t.Fatal(err)
	}
	k.Close()
	defer cleanupTestKey()

	elog = new(testCabbieLog)
	expected := newSettings()
	expected.RequiredCategories = rc
	testconfig := newSettings()
	// End Setup
	if err := testconfig.regLoad(testPath); err != nil {
		t.Error(err)
	}
	if !(cmp.Equal(testconfig, expected)) {
		t.Errorf("testconfig.regLoad(%s) = %v, want %v", testPath, testconfig, expected)
	}
}
