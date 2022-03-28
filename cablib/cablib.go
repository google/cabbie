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

// Package cablib is a library of shared constants and functions.
package cablib

import (
	"fmt"
	"os"
	"reflect"
	"time"
)

const (
	// S_OK is the return HResult for successful method calls.
	S_OK = 0x00000000
	// LogSrcName is the name of event log source.
	LogSrcName = "Cabbie"
	// SvcName is the name of the registered Service.
	SvcName = "Cabbie"
	// CabbieExe is the windows path to the cabbie executable.
	CabbieExe = `cabbie.exe`
	// CabbiePath is the Windows path to the cabbie files.
	CabbiePath = `C:\Program Files\Google\Cabbie\`
	// WUReg is the registry path to the local update client configuration.
	WUReg = `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`
	// MetricSvc is service name of a metric.
	MetricSvc = "Cabbie"
	// MetricRoot is the root path for a metric.
	MetricRoot = `Cabbie\metrics`

	rebootValue = "RebootTime"
)

var (
	now            = time.Now
	// RegPath is the registry path to the cabbie settings.
	RegPath = `SOFTWARE\Google\Cabbie\`
)

// StringInSlice checks if a slice contains a string.
func StringInSlice(e string, s []string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// SetField sets the value of a struct field based on the field name.
func SetField(obj interface{}, name string, value interface{}) error {
	structValue := reflect.ValueOf(obj).Elem()
	structFieldValue := structValue.FieldByName(name)

	if !structFieldValue.IsValid() {
		return fmt.Errorf("no such field: %s in obj", name)
	}

	if !structFieldValue.CanSet() {
		return fmt.Errorf("cannot set %s field value", name)
	}

	structFieldType := structFieldValue.Type()
	val := reflect.ValueOf(value)

	if structFieldType.AssignableTo(val.Type()) {
		structFieldValue.Set(val)
		return nil
	}

	return fmt.Errorf("provided value type (%v) didn't match obj field type (%v)", val.Type(), structFieldType)
}

// FileExists used for determining if given file exists.
func FileExists(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("fileExists: received empty string to test")
	}
	p, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return !p.IsDir(), nil
}

// PathExists used for determining if given path exists.
func PathExists(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("pathExists: received empty string to test")
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// SliceContains evaluates if a given value is in the passed slice.
func SliceContains(slice interface{}, v interface{}) bool {
	list := reflect.ValueOf(slice)
	for i := 0; i < list.Len(); i++ {
		if list.Index(i).Interface() == v {
			return true
		}
	}
	return false
}
