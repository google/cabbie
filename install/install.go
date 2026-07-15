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

// Package install handles installing updates.
package install

import (
	"golang.org/x/net/context"
	goerr "errors"
	"fmt"
	"sync"

	"github.com/google/cabbie/errors"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

var (
	// ErrBusy indicates that the Windows installer is busy
	ErrBusy = goerr.New("an installation or uninstallation is already in progress")
)

// Installer represents an update Install interface.
// https://docs.microsoft.com/en-us/windows/desktop/api/wuapi/nn-wuapi-iupdateinstaller
type Installer struct {
	mu                  sync.Mutex
	IUpdateInstaller    *ole.IDispatch
	IInstallationResult *ole.IDispatch
}

// NewInstaller creates an update download interface with a specified update collection.
func NewInstaller(us *session.UpdateSession, uc *updatecollection.Collection) (*Installer, error) {
	udi, err := us.CreateInterface(session.Installer)
	if err != nil {
		return nil, err
	}

	if _, err = oleutil.PutProperty(udi, "Updates", uc.IUpdateCollection); err != nil {
		udi.Release()
		return nil, fmt.Errorf("failed to register updates for install: \n %v", err)
	}

	return &Installer{IUpdateInstaller: udi}, nil
}

func (i *Installer) isBusyLocked() (bool, error) {
	if i.IUpdateInstaller == nil {
		return false, fmt.Errorf("nil IUpdateInstaller dispatch")
	}
	p, err := oleutil.GetProperty(i.IUpdateInstaller, "IsBusy")
	if err != nil {
		if p != nil {
			p.Clear()
		}
		return false, err
	}
	if p == nil {
		return false, fmt.Errorf("error getting IsBusy: nil variant returned")
	}
	defer p.Clear()
	return p.Value().(bool), nil
}

// IsBusy gets a Boolean value that indicates whether an installation or uninstallation is in progress.
func (i *Installer) IsBusy() (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.isBusyLocked()
}

// Install will install the requested updates.
func (i *Installer) Install(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.IUpdateInstaller == nil {
		return fmt.Errorf("install error: nil IUpdateInstaller dispatch")
	}
	b, _ := i.isBusyLocked()
	if b {
		return ErrBusy
	}
	r, err := oleutil.CallMethod(i.IUpdateInstaller, "Install")
	if err != nil {
		var val int64
		if r != nil {
			val = r.Val
			r.Clear()
		}
		return fmt.Errorf("install error: [%s] [%v]", errors.UpdateError(val), err)
	}
	if r == nil {
		return fmt.Errorf("install error: nil variant returned")
	}
	i.IInstallationResult = r.ToIDispatch()
	return nil
}

// Uninstall starts a synchronous uninstallation of the updates.
func (i *Installer) Uninstall(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.IUpdateInstaller == nil {
		return fmt.Errorf("uninstall error: nil IUpdateInstaller dispatch")
	}
	r, err := oleutil.CallMethod(i.IUpdateInstaller, "Uninstall")
	if err != nil {
		var val int64
		if r != nil {
			val = r.Val
			r.Clear()
		}
		return fmt.Errorf("uninstall error: [%s] [%v]", errors.UpdateError(val), err)
	}
	if r == nil {
		return fmt.Errorf("uninstall error: nil variant returned")
	}
	i.IInstallationResult = r.ToIDispatch()
	return nil
}

// Commit finalizes updates that were previously staged or installed.
// https://learn.microsoft.com/en-us/windows/win32/api/wuapi/nf-wuapi-iupdateinstaller4-commit
func (i *Installer) Commit(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.IUpdateInstaller == nil {
		return fmt.Errorf("commit error: nil IUpdateInstaller dispatch")
	}
	// dwFlags is reserved for future use; currently passing a blank variable.
	var dwFlags uint32
	r, err := oleutil.CallMethod(i.IUpdateInstaller, "Commit", dwFlags)
	if r != nil {
		defer r.Clear()
	}
	if err != nil {
		var val int64
		if r != nil {
			val = r.Val
		}
		return fmt.Errorf("commit error: [%s] [%v]", errors.UpdateError(val), err)
	}
	return nil
}

// HResult gets the HRESULT of the exception, if any, that is raised during the installation.
func (i *Installer) HResult() (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.IInstallationResult == nil {
		return "", fmt.Errorf("error getting HResult property: nil IInstallationResult dispatch")
	}
	hr, err := oleutil.GetProperty(i.IInstallationResult, "HResult")
	if err != nil {
		if hr != nil {
			hr.Clear()
		}
		return "", fmt.Errorf("error getting HResult property: %v", err)
	}
	if hr == nil {
		return "", fmt.Errorf("error getting HResult property: nil variant returned")
	}
	defer hr.Clear()
	return fmt.Sprintf("%s", errors.UpdateError(hr.Val)), nil
}

// ResultCode gets an OperationResultCode value that specifies the result of an operation on an update.
// Possible Result codes:
// 0 - (orcNotStarted)	The operation is not started.
// 1 - (orcInProgress)	The operation is in progress.
// 2 - (orcSucceeded)	The operation was completed successfully.
// 3 - (orcSucceededWithErrors)	The operation is complete, but one or more errors occurred during the operation. The results might be incomplete.
// 4 - (orcFailed)	The operation failed to complete.
// 5 - (orcAborted)	The operation is canceled.
func (i *Installer) ResultCode() (int, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.IInstallationResult == nil {
		return 0, fmt.Errorf("error getting ResultCode property: nil IInstallationResult dispatch")
	}
	rc, err := oleutil.GetProperty(i.IInstallationResult, "ResultCode")
	if err != nil {
		if rc != nil {
			rc.Clear()
		}
		return 0, fmt.Errorf("error getting ResultCode property: %v", err)
	}
	if rc == nil {
		return 0, fmt.Errorf("error getting ResultCode property: nil variant returned")
	}
	defer rc.Clear()
	return int(rc.Val), nil
}

// RebootRequired gets a Boolean value that indicates whether you must restart the computer to complete the installation.
func (i *Installer) RebootRequired() (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.IInstallationResult == nil {
		return false, fmt.Errorf("error getting reboot required property: nil IInstallationResult dispatch")
	}
	rr, err := oleutil.GetProperty(i.IInstallationResult, "RebootRequired")
	if err != nil {
		if rr != nil {
			rr.Clear()
		}
		return false, fmt.Errorf("error getting reboot required property: %v", err)
	}
	if rr == nil {
		return false, fmt.Errorf("error getting reboot required property: nil variant returned")
	}
	defer rr.Clear()
	return rr.Value().(bool), nil
}

// Close turns down any open download sessions.
func (i *Installer) Close() {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.IUpdateInstaller != nil {
		i.IUpdateInstaller.Release()
		i.IUpdateInstaller = nil
	}
	if i.IInstallationResult != nil {
		i.IInstallationResult.Release()
		i.IInstallationResult = nil
	}
}

