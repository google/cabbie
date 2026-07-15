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

//go:build windows
// +build windows

package cablib

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/windows/registry"
)

const (
	testPath = `SOFTWARE\Bar`
)

var (
	fakeUpdates = []string{"123456", "234567"}
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

func TestAddGetRebootUpdatesSuccess(t *testing.T) {
	// Setup
	SetRegPath(testPath)
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	// End Setup
	if err := AddRebootUpdates(fakeUpdates); err != nil {
		t.Fatal(err)
	}
	got, err := GetRebootUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(got, fakeUpdates) {
		t.Errorf("GetRebootUpdates() = %v, want %v", got, fakeUpdates)
	}
}

func TestAddGetRebootUpdatesFailure(t *testing.T) {
	// Setup
	SetRegPath(testPath)
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	// End Setup
	if err := AddRebootUpdates(fakeUpdates); err != nil {
		t.Fatal(err)
	}
	if err := cleanRebootUpdatesValue(); err != nil {
		t.Fatal(err)
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath(), registry.READ)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	if _, _, err = k.GetStringsValue(`RebootUpdates`); err != registry.ErrNotExist || err == nil {
		t.Fatal(err)
	}
}

func TestSetRebootTime(t *testing.T) {
	// Setup
	SetNowFunc(fakeTimeNow)
	SetRegPath(testPath)
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	// End Setup
	for _, tt := range []struct {
		in  time.Time
		val []byte
	}{
		{Now().Add(time.Second * time.Duration(200)), []byte{1, 0, 0, 0, 14, 194, 149, 0, 186, 38, 211, 97, 101, 255, 255}},
		{Now(), []byte{1, 0, 0, 0, 14, 194, 148, 255, 242, 38, 211, 97, 101, 255, 255}},
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
	SetRebootRequiredFunc(testRebootFalse)
	SetRegPath(testPath)
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
	SetNowFunc(fakeTimeNow)
	SetRegPath(testPath)
	SetRebootRequiredFunc(testRebootFalse)
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	if err := setBinarykey(Now()); err != nil {
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
	SetNowFunc(fakeTimeNow)
	SetRegPath(testPath)
	SetRebootRequiredFunc(testRebootTrue)
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()

	if err := setBinarykey(Now()); err != nil {
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
		o := SliceContains(tt.sl, tt.st)
		if o != tt.out {
			t.Errorf("SliceContains(%v, %q) = %t, want %t", tt.sl, tt.st, o, tt.out)
		}
	}

	for _, tt := range []struct {
		sl  []int
		st  int
		out bool
	}{
		{[]int{123, 98}, 123, true},
		{[]int{4567, 2000, 8}, 2, false},
		{[]int{}, 334, false},
	} {
		o := SliceContains(tt.sl, tt.st)
		if o != tt.out {
			t.Errorf("SliceContains(%v, %d) = %t, want %t", tt.sl, tt.st, o, tt.out)
		}
	}
}

type dummyStruct struct {
	Exported   string
	unexported string
}

func TestSetField_EdgeCasesAndPanics(t *testing.T) {
	t.Run("nil obj panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic when obj is nil, but code did not panic")
			}
		}()
		_ = SetField(nil, "Exported", "val")
	})

	t.Run("non-pointer obj panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic when obj is struct value, but code did not panic")
			}
		}()
		d := dummyStruct{}
		_ = SetField(d, "Exported", "val")
	})

	t.Run("nil value panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic when value is nil, but code did not panic")
			}
		}()
		d := &dummyStruct{}
		_ = SetField(d, "Exported", nil)
	})

	t.Run("unexported field error", func(t *testing.T) {
		d := &dummyStruct{}
		err := SetField(d, "unexported", "val")
		if err == nil {
			t.Errorf("expected error setting unexported field, got nil")
		}
	})

	t.Run("nonexistent field error", func(t *testing.T) {
		d := &dummyStruct{}
		err := SetField(d, "NonExistent", "val")
		if err == nil {
			t.Errorf("expected error setting non-existent field, got nil")
		}
	})

	t.Run("type mismatch error", func(t *testing.T) {
		d := &dummyStruct{}
		err := SetField(d, "Exported", 12345)
		if err == nil {
			t.Errorf("expected error setting string field with int value, got nil")
		}
	})
}

func TestSystemReboot_NilContext(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SystemReboot with nil context panicked: %v", r)
		}
	}()
	SetSleepWaitTime(1 * time.Millisecond)
	defer SetSleepWaitTime(30 * time.Minute)
	_ = SystemReboot(nil, time.Now().Add(-1*time.Minute))
}

func TestRegPath_Concurrency(t *testing.T) {
	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				if j%2 == 0 {
					SetRegPath(fmt.Sprintf("SOFTWARE\\TestPath_%d_%d", id, j))
				} else {
					_ = RegPath()
				}
			}
			done <- true
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestNow_Concurrency(t *testing.T) {
	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				if j%2 == 0 {
					SetNowFunc(func() time.Time { return time.Date(2020+id, 1, 1, 0, 0, 0, 0, time.UTC) })
				} else {
					_ = Now()
				}
			}
			done <- true
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

func TestFileAndPathExists_EdgeCases(t *testing.T) {
	if _, err := FileExists(""); err == nil {
		t.Errorf("FileExists(\"\") expected error, got nil")
	}
	if _, err := PathExists(""); err == nil {
		t.Errorf("PathExists(\"\") expected error, got nil")
	}
}


