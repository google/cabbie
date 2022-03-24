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

// Package servicemgr adds or removes the registration of the update service with
// Windows Update Agent.
package servicemgr

import (
	"github.com/go-ole/go-ole"
)

// ServiceID indicates which update source is being scanned. More info and common
// ServiceIDs can be found here:
//
//   https://docs.microsoft.com/en-us/windows/deployment/update/how-windows-update-works
type ServiceID string

// ServiceManager describes an update service manager COM object.
// https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdateservicemanager
type ServiceManager struct {
	ServiceManager *ole.IDispatch
}

const (
	notRegistered = iota + 1
	pending
	registered

	// Default is the Unspecified/Default	serviceID.
	Default ServiceID = "00000000-0000-0000-0000-000000000000"
	// WindowsUpdate ServiceID.
	WindowsUpdate ServiceID = "9482f4b4-e343-43b6-b170-9a65bc822c77"
	// MicrosoftUpdate ServiceID.
	MicrosoftUpdate ServiceID = "7971f918-a847-4430-9279-4a52d1efe18d"
	// WindowsStore ServiceID.
	WindowsStore ServiceID = "855E8A7C-ECB4-4CA3-B045-1DFA50104289"
	// WSUS ServiceID.
	WSUS ServiceID = "3DA21691-E39D-4da6-8A4B-B43877BCB1B7"
)
