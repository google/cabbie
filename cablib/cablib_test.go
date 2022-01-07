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
package cablib

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	testPath = `SOFTWARE\Bar`
)

var (
	fakeTimeNow = func() time.Time {
		return time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
	}
	testRebootTrue  = func() (bool, error) { return true, nil }
	testRebootFalse = func() (bool, error) { return false, nil }
)

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

func getBinarykey(v string) ([]byte, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, testPath, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	r, _, err := k.GetBinaryValue(v)
	return r, err
}

func setBinarykey(t time.Time) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, testPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	b, err := t.MarshalBinary()
	if err != nil {
		return err
	}
	return k.SetBinaryValue(rebootValue, b)
}

func TestSetRebootTime(t *testing.T) {
	// Setup
	now = fakeTimeNow
	RegPath = testPath
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	// End Setup
	for _, tt := range []struct {
		in  uint64
		val []byte
	}{
		{uint64(200), []byte{1, 0, 0, 0, 14, 194, 149, 0, 186, 38, 211, 97, 101, 255, 255}},
		{uint64(0), []byte{1, 0, 0, 0, 14, 194, 148, 255, 242, 38, 211, 97, 101, 255, 255}},
	} {
		err := SetRebootTime(tt.in)
		if err != nil {
			t.Error(err)
		}
		r, err := getBinarykey(rebootValue)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(r, tt.val) {
			fmt.Println(r)
			t.Errorf("SetRebootTime(%v) = %X, want %X", tt.in, r, tt.val)
		}
	}
}

func TestRebootTimeMissingValue(t *testing.T) {
	// Setup
	rebootRequired = testRebootFalse
	RegPath = testPath
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	// End Setup

	tt, err := RebootTime()
	if err != nil {
		t.Error(err)
	}
	if !(tt.IsZero()) {
		t.Errorf("got %s, want %s", tt, time.Time{})
	}
}

func TestRebootTimeNoReboot(t *testing.T) {
	// Setup
	now = fakeTimeNow
	RegPath = testPath
	rebootRequired = testRebootFalse
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	if err := setBinarykey(now()); err != nil {
		t.Fatal(err)
	}
	// End Setup

	tt, err := RebootTime()
	if err != nil {
		t.Error(err)
	}
	if !(tt.IsZero()) {
		t.Errorf("RebootTime() = %s, wanted %s", tt, time.Time{})
	}

	_, err = getBinarykey(rebootValue)
	if err != registry.ErrNotExist {
		t.Errorf("Registry value %q still found, expected missing", rebootValue)
	}
}

func TestRebootTimeSuccess(t *testing.T) {
	// Setup
	now = fakeTimeNow
	RegPath = testPath
	rebootRequired = testRebootTrue
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()

	if err := setBinarykey(now()); err != nil {
		t.Fatal(err)
	}
	r, err := getBinarykey(rebootValue)
	var tval time.Time
	tval.UnmarshalBinary(r)
	fmt.Println(tval)
	if err != nil {
		t.Fatal(err)
	}
	// End Setup
	tt, err := RebootTime()
	if err != nil {
		t.Error(err)
	}
	if tt.IsZero() {
		t.Errorf("RebootTime(%s) = %v, wanted non-Zero time", tval, tt)
	}
}

func TestStringInSlice(t *testing.T) {
	for _, tt := range []struct {
		sl  []string
		st  string
		out bool
	}{
		{[]string{"abc"}, "abc", true},
		{[]string{"abc"}, "ab", false},
		{[]string{"abc"}, "", false},
		{[]string{"abc"}, "def", false},
		{[]string{"123", "abc", "def"}, "def", true},
		{[]string{"", "abc", "def"}, "df", false},
		{[]string{}, "df", false},
		{[]string{"   "}, "df", false},
	} {
		o := StringInSlice(tt.st, tt.sl)
		if o != tt.out {
			t.Errorf("got %t, want %t", o, tt.out)
		}
	}
}

func TestSliceContains(t *testing.T) {
	for _, tt := range []struct {
		sl  interface{}
		st  interface{}
		out bool
	}{
		{[]string{"abc"}, "abc", true},
		{[]string{"abc"}, "ab", false},
		{[]string{"abc"}, "", false},
		{[]string{"abc"}, "def", false},
		{[]string{"123", "abc", "def"}, "def", true},
		{[]string{"", "abc", "def"}, "df", false},
		{[]string{}, "df", false},
		{[]string{"   "}, "df", false},
		{[]int{123, 98}, 123, true},
		{[]int{4567, 2000, 8}, 2, false},
		{[]int{}, 334, false},
	} {
		o := SliceContains(tt.sl, tt.st)
		if o != tt.out {
			t.Errorf("got %t, want %t", o, tt.out)
		}
	}
}
