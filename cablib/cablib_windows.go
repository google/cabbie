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

//go:build windows
// +build windows

package cablib

import (
	"golang.org/x/net/context"
	"fmt"
	"sync"
	"time"

	"github.com/google/cabbie/notification"
	"golang.org/x/sys/windows/registry"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/google/glazier/go/power"
)

var (
	rebootRequiredMu   sync.RWMutex
	rebootRequiredFunc = RebootRequired

	// IIDIWindowsDriverUpdate is the GUID for the IWindowsDriverUpdate COM interface.
	// See: https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-uamg/e839e7e0-1795-451b-94ef-abacd6cbecac
	IIDIWindowsDriverUpdate = ole.NewGUID("B383CD1A-5CE9-4504-9F63-764B1236F191")

	sleepWaitTimeMu sync.RWMutex
	sleepWaitTime   = 30 * time.Minute
)

// SetRebootRequiredFunc sets a custom function to determine if reboot is required (for testing).
func SetRebootRequiredFunc(f func() (bool, error)) {
	rebootRequiredMu.Lock()
	defer rebootRequiredMu.Unlock()
	rebootRequiredFunc = f
}

// SetSleepWaitTime sets the popup sleep duration (for testing).
func SetSleepWaitTime(d time.Duration) {
	sleepWaitTimeMu.Lock()
	defer sleepWaitTimeMu.Unlock()
	sleepWaitTime = d
}

func getSleepWaitTime() time.Duration {
	sleepWaitTimeMu.RLock()
	defer sleepWaitTimeMu.RUnlock()
	return sleepWaitTime
}

func getRebootRequired() (bool, error) {
	rebootRequiredMu.RLock()
	defer rebootRequiredMu.RUnlock()
	return rebootRequiredFunc()
}

// AddRebootUpdates adds a reboot-required update list of KBs to the registry.
func AddRebootUpdates(kbs []string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath(), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.SetStringsValue(`RebootUpdates`, kbs)
}

// GetRebootUpdates retrieves the reboot-required update list of KBs from the registry.
func GetRebootUpdates() ([]string, error) {
	var kbs []string
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath(), registry.READ)
	if err != nil {
		return kbs, err
	}
	defer k.Close()

	kbs, _, err = k.GetStringsValue(`RebootUpdates`)
	if err != nil && err != registry.ErrNotExist {
		return kbs, fmt.Errorf("unable to get updates requiring reboot: %v", err)
	}

	return kbs, nil
}

// cleanRebootUpdatesValue clears the reboot-required update list from the registry.
func cleanRebootUpdatesValue() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath(), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.DeleteValue(`RebootUpdates`)
}

// SetInstallAtShutdown sets a flag to force Feature Updates to install on reboot to the registry.
// https://dennisbabkin.com/blog/?t=how-to-enable-installation-of-updates-or-to-prevent-it-during-reboot-or-shutdown
func SetInstallAtShutdown() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Orchestrator\`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.SetDWordValue(`InstallAtShutdown`, 1)
}

// cleanInstallAtShutdownValue removes a flag to force Feature Updates to install on reboot to the registry.
func cleanInstallAtShutdownValue() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Orchestrator\`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.DeleteValue(`InstallAtShutdown`)
}

// SetRebootTime creates the reboot time key.
func SetRebootTime(t time.Time) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath(), registry.SET_VALUE)
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

// RebootTime gets the value of "rebootValue" from the registry.
func RebootTime() (time.Time, error) {
	var t time.Time
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath(), registry.ALL_ACCESS)
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
	rbr, err := getRebootRequired()
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

// ClearRebootTime deletes the reboot time key.
func ClearRebootTime() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, RegPath(), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.DeleteValue(rebootValue)
}

// SystemReboot initiates a restart when the set reboot time has passed.
func SystemReboot(ctx context.Context, t time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-time.After(time.Until(t)):
	case <-ctx.Done():
		return ctx.Err()
	}

	notification.RebootPopup(30).Push(ctx)

	select {
	case <-time.After(getSleepWaitTime()):
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := ClearRebootTime(); err != nil {
		return fmt.Errorf("failed to clean up registry value %q: %v", rebootValue, err)
	}
	if err := cleanRebootUpdatesValue(); err != nil {
		if err != registry.ErrNotExist {
			return fmt.Errorf("failed to clear updates requiring reboot from registry: %v", err)
		}
	}
	return power.Reboot(power.SHTDN_REASON_MAJOR_SOFTWARE, true)
}

// Count gets the count property of an IDispatch object.
func Count(id *ole.IDispatch) (int, error) {
	count, err := oleutil.GetProperty(id, "Count")
	if count != nil {
		defer count.Clear()
	}
	if err != nil {
		return 0, fmt.Errorf("error getting update count, %v", err)
	}
	if count == nil {
		return 0, fmt.Errorf("error getting update count: nil variant returned")
	}
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
	if r != nil {
		defer r.Clear()
	}
	if err != nil {
		return false, fmt.Errorf("failed to get RebootRequired property: %v", err)
	}
	if r == nil {
		return false, fmt.Errorf("failed to get RebootRequired property: nil variant returned")
	}

	return r.Value().(bool), nil
}

func getUpdateTitleAt(collection *ole.IDispatch, index int) (string, error) {
	item, err := oleutil.GetProperty(collection, "item", index)
	if item != nil {
		defer item.Clear()
	}
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", fmt.Errorf("nil item variant returned for index %d", index)
	}

	itemd := item.ToIDispatch()

	title, err := oleutil.GetProperty(itemd, "Title")
	if title != nil {
		defer title.Clear()
	}
	if err != nil {
		return "", err
	}
	if title == nil {
		return "", fmt.Errorf("nil title variant returned for index %d", index)
	}

	return title.ToString(), nil
}

// GetUpdateTitles loops through an update collection and returns a list of titles.
func GetUpdateTitles(collection *ole.IDispatch, count int) ([]string, []error) {
	var errors []error
	var u []string

	for i := 0; i < count; i++ {
		title, err := getUpdateTitleAt(collection, i)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		u = append(u, title)
	}

	if len(errors) > 0 {
		return nil, errors
	}
	return u, nil
}

