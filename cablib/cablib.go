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
	"reflect"
	"time"

	"github.com/google/cabbie/notification"
	"golang.org/x/sys/windows/registry"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/google/glazier/go/power"
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
	rebootRequired = RebootRequired
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

// SetRebootTime creates the reboot time key.
func SetRebootTime(seconds uint64) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	t := now().Add(time.Second * time.Duration(seconds))
	b, err := t.MarshalBinary()
	if err != nil {
		return err
	}
	return k.SetBinaryValue(rebootValue, b)
}

// RebootTime gets the value of "rebootValue" from the registry.
func RebootTime() (time.Time, error) {
	var t time.Time
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath, registry.ALL_ACCESS)
	if err != nil {
		return t, err
	}
	defer k.Close()

	b, _, err := k.GetBinaryValue(rebootValue)
	if err != nil {
		if err == registry.ErrNotExist {
			return t, nil
		}
		return t, fmt.Errorf("unable to get scheduled reboot time: %v", err)
	}

	// Remove timer if no longer pending a reboot.
	rbr, err := rebootRequired()
	if err != nil {
		return t, err
	}
	if !rbr {
		return t, k.DeleteValue(rebootValue)
	}

	if err := t.UnmarshalBinary(b); err != nil {
		return t, fmt.Errorf("unable to Unmarshal binary data: %v", err)
	}

	return t, nil
}

func cleanRebootValue() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.DeleteValue(rebootValue)
}

// SystemReboot initates a restart when the set reboot time has passed. This should be called within a goroutine
func SystemReboot(t time.Time) error {
	time.Sleep(time.Until(t))

	notification.RebootPopup(2).Push()

	time.Sleep(2 * time.Minute)

	if err := cleanRebootValue(); err != nil {
		return fmt.Errorf("failed to clean up registry value %q: %v", rebootValue, err)
	}
	return power.Reboot(power.SHTDN_REASON_MAJOR_SOFTWARE, true)
}

// Count gets the count property of an IDispatch object.
func Count(id *ole.IDispatch) (int, error) {
	count, err := oleutil.GetProperty(id, "Count")
	if err != nil {
		return 0, fmt.Errorf("error getting update count, %v", err)
	}
	defer count.Clear()
	return int(count.Val), nil
}

// NewCOMObject creates a new COM object for the specifed ProgramID.
func NewCOMObject(id string) (*ole.IDispatch, error) {
	unknown, err := oleutil.CreateObject(id)
	if err != nil {
		return nil, fmt.Errorf("unable to create initial unknown object: %v", err)
	}
	defer unknown.Release()

	obj, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("Unable to create query interface: %v", err)
	}

	return obj, nil
}

// RebootRequired indicates whether a system restart is required.
func RebootRequired() (bool, error) {
	sysinfo, err := NewCOMObject("Microsoft.Update.SystemInfo")
	if err != nil {
		return false, err
	}
	defer sysinfo.Release()

	r, err := oleutil.GetProperty(sysinfo, "RebootRequired")
	if err != nil {
		return false, fmt.Errorf("failed to get RebootRequired property: %v", err)
	}
	defer r.Clear()

	return r.Value().(bool), nil
}

// GetUpdateTitles loops through an update collection and returns a list of titles.
func GetUpdateTitles(collection *ole.IDispatch, count int) ([]string, []error) {
	var errors []error
	var u []string

	for i := 0; i < count; i++ {
		// Get update at position i
		item, err := oleutil.GetProperty(collection, "item", i)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		itemd := item.ToIDispatch()

		// Get selected updates title
		title, err := oleutil.GetProperty(itemd, "Title")
		if err != nil {
			errors = append(errors, err)
			continue
		}

		u = append(u, title.ToString())
		itemd.Release()
		title.Clear()
	}

	if len(errors) > 0 {
		return nil, errors
	}
	return u, nil
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
