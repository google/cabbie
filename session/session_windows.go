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

// Package session represents an update session in which the caller can perform operations that involve updates.
// More info: https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdatesession
package session

import (
	"fmt"

	"github.com/google/cabbie/cablib"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// New creates an update session object.
func New() (*UpdateSession, error) {

	session, err := cablib.NewCOMObject("Microsoft.Update.Session")
	if err != nil {
		return nil, fmt.Errorf("failed to create new COM object: %v", err)
	}
	oleutil.PutProperty(session, "ClientApplicationID", clientID)
	return &UpdateSession{Session: session}, nil
}

// CreateInterface creates the requested update interface.
// updateInterface can be one of: Searcher, Downloader, or Installer.
func (u *UpdateSession) CreateInterface(ui updateInterface) (*ole.IDispatch, error) {
	us, err := oleutil.CallMethod(u.Session, string(ui))
	if err != nil {
		return nil, fmt.Errorf("error creating requested interface: %v", err)
	}
	return us.ToIDispatch(), nil
}

// Close turns down any open update sessions.
func (u *UpdateSession) Close() {
	u.Session.Release()
}
