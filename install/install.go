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

// Package install handles installing updates.
package install

import (
	goerr "errors"
	"fmt"

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
		return nil, fmt.Errorf("failed to register updates for install: \n %v", err)
	}

	return &Installer{IUpdateInstaller: udi}, nil
}

// Install will install the requested updates.
func (i *Installer) Install() error {
	b, _ := i.IsBusy()
	if b {
		return ErrBusy
	}
	r, err := oleutil.CallMethod(i.IUpdateInstaller, "Install")
	i.IInstallationResult = r.ToIDispatch()
	if err != nil {
		return fmt.Errorf("install error: [%s] [%v]", errors.UpdateError(r.Val), err)
	}
	return nil
}

// Uninstall starts a synchronous uninstallation of the updates.
func (i *Installer) Uninstall() error {
	r, err := oleutil.CallMethod(i.IUpdateInstaller, "Uninstall")
	i.IInstallationResult = r.ToIDispatch()
	if err != nil {
		return fmt.Errorf("uninstall error: [%s] [%v]", errors.UpdateError(r.Val), err)
	}
	return nil
}

// IsBusy gets a Boolean value that indicates whether an installation or uninstallation is in progress.
func (i *Installer) IsBusy() (bool, error) {
	p, err := oleutil.GetProperty(i.IUpdateInstaller, "IsBusy")
	if err != nil {
		return false, err
	}
	return p.Value().(bool), nil
}

// HResult gets the HRESULT of the exception, if any, that is raised during the installation.
func (i *Installer) HResult() (string, error) {
	hr, err := oleutil.GetProperty(i.IInstallationResult, "HResult")
	if err != nil {
		return "", fmt.Errorf("error getting HResult property: %v", err)
	}
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
	rc, err := oleutil.GetProperty(i.IInstallationResult, "ResultCode")
	if err != nil {
		return 0, fmt.Errorf("error getting ResultCode property: %v", err)
	}
	return int(rc.Val), nil
}

// RebootRequired gets a Boolean value that indicates whether you must restart the computer to complete the installation.
func (i *Installer) RebootRequired() (bool, error) {
	rr, err := oleutil.GetProperty(i.IInstallationResult, "RebootRequired")
	if err != nil {
		return false, fmt.Errorf("error getting reboot required property: %v", err)
	}
	return rr.Value().(bool), nil
}

// Close turns down any open download sessions.
func (i *Installer) Close() {
	i.IUpdateInstaller.Release()
	if i.IInstallationResult != nil {
		i.IInstallationResult.Release()
	}
}
