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

package servicemgr

import (
	"github.com/google/cabbie/cablib"
	"github.com/go-ole/go-ole/oleutil"
)

// InitMgrService creates an update service manager object.
func InitMgrService() (*ServiceManager, error) {
	mgr, err := cablib.NewCOMObject("Microsoft.Update.ServiceManager")
	if err != nil {
		return nil, err
	}
	return &ServiceManager{ServiceManager: mgr}, nil
}

// AddService registers a service with Windows Update Agent (WUA) without requiring an authorization
// cabinet file (.cab).
// More info can be found at https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nf-wuapi-iupdateservicemanager2-addservice2
func (m *ServiceManager) AddService(s ServiceID) error {
	_, err := oleutil.CallMethod(m.ServiceManager, "AddService2", string(s), 7, "")
	return err
}

// QueryServiceRegistration verifies if a serviceID has been registered with Windows Update Agent.
func (m *ServiceManager) QueryServiceRegistration(s ServiceID) (bool, error) {
	sr, err := oleutil.CallMethod(m.ServiceManager, "QueryServiceRegistration", string(s))
	if err != nil {
		return false, err
	}
	srd := sr.ToIDispatch()
	defer srd.Release()

	state, err := oleutil.GetProperty(srd, "RegistrationState")
	if err != nil {
		return false, err
	}
	defer state.Clear()

	// Possible state values:
	// 1 = The service is not registered.
	// 2 = The service is pending registration.
	// 3 = The service is registered.
	if state.Val == registered {
		return true, nil
	}

	return false, nil
}

// RemoveService removes a service registration from Windows Update Agent (WUA).
func (m *ServiceManager) RemoveService(s ServiceID) error {
	_, err := oleutil.CallMethod(m.ServiceManager, "RemoveService", string(s))
	return err
}

// Close turns down any open service manager sessions.
func (m *ServiceManager) Close() {
	m.ServiceManager.Release()
}
