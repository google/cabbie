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

// Package session represents an update session in which the caller can perform operations that involve updates.
// More info: https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdatesession
package session

import (
	"github.com/go-ole/go-ole"
)

type updateInterface string

// UpdateSession describes an update session COM object.
type UpdateSession struct {
	Session *ole.IDispatch
}

const (
	clientID = "Cabbie Windows Update API"
	// Searcher is the method name to create an update searcher interface.
	Searcher updateInterface = "CreateUpdateSearcher"
	// Downloader is the method name to create an update downloader interface.
	Downloader updateInterface = "CreateUpdateDownloader"
	// Installer is the method name to create an update installer interface.
	Installer updateInterface = "CreateUpdateInstaller"
)
