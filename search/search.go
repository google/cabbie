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

// +build windows

// Package search handles querying for Windows updates.
package search

import (
	"fmt"

	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/errors"
	"github.com/google/cabbie/servicemgr"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
	"github.com/google/cabbie/updates"
	"github.com/google/cabbie/wsus"
	"golang.org/x/sys/windows/registry"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// CategoryID represents the category to which an update belongs.
// GUIDs can be found here:
// https://docs.microsoft.com/en-us/previous-versions/windows/desktop/ff357803(v=vs.85)?redirectedfrom=MSDN
type CategoryID string

const (
	// Application GUID
	Application CategoryID = "5C9376AB-8CE6-464A-B136-22113DD69801"
	// Connectors GUID
	Connectors CategoryID = "434DE588-ED14-48F5-8EED-A15E09A991F6"
	// CriticalUpdates GUID
	CriticalUpdates CategoryID = "E6CF1350-C01B-414D-A61F-263D14D133B4"
	// DefinitionUpdates GUID
	DefinitionUpdates CategoryID = "E0789628-CE08-4437-BE74-2495B842F43B"
	// DeveloperKits GUID
	DeveloperKits CategoryID = "E140075D-8433-45C3-AD87-E72345B36078"
	// Drivers GUID
	Drivers CategoryID = "EBFC1FC5-71A4-4F7B-9ACA-3B9A503104A0"
	// FeaturePacks GUID
	FeaturePacks CategoryID = "B54E7D24-7ADD-428F-8B75-90A396FA584F"
	// Guidance GUID
	Guidance CategoryID = "9511D615-35B2-47BB-927F-F73D8E9260BB"
	// SecurityUpdates GUID
	SecurityUpdates CategoryID = "0FA1201D-4330-4FA8-8AE9-B877473B6441"
	// ServicePacks GUID
	ServicePacks CategoryID = "68C5B0A3-D1A6-4553-AE49-01D3A7827828"
	// Tools GUID
	Tools CategoryID = "B4832BD8-E735-4761-8DAF-37F882276DAB"
	// UpdateRollups GUID
	UpdateRollups CategoryID = "28BC880E-0592-4CBF-8F95-C79B17911D5F"
	// Updates GUID
	Updates CategoryID = "CD5FFD1E-E932-4E3A-BF74-18BF0B1BBD83"
	// Upgrades GUID
	Upgrades CategoryID = "3689BDC8-B205-4AF4-8D4A-A63924C5E9D5"
	// BasicSearch is the default search to query for assigned updates that are not installed
	BasicSearch = "IsInstalled=0 and DeploymentAction='Installation'"
)

// Searcher describes search properties
// ISearchResult interface be found here: https://docs.microsoft.com/en-us/windows/desktop/api/wuapi/nn-wuapi-isearchresult
type Searcher struct {
	IUpdateSearcher                     *ole.IDispatch
	Criteria                            string
	ServerSelection                     int
	SearchScope                         int
	IncludePotentiallySupersededUpdates bool
	ClientApplicationID                 string
	ServiceID                           string
	SearchHResult                       string
	ISearchResult                       *ole.IDispatch
}

func (s *Searcher) configureRegistry() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, cablib.WUReg, registry.SET_VALUE)
	if err != nil && err != registry.ErrNotExist {
		return err
	}
	if err == registry.ErrNotExist {
		k, _, err = registry.CreateKey(registry.LOCAL_MACHINE, cablib.WUReg, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
	}
	defer k.Close()

	if s.ServerSelection == wsus.ManagedServer {
		return k.SetDWordValue("DoNotConnectToWindowsUpdateInternetLocations", 1)
	}

	return k.SetDWordValue("DoNotConnectToWindowsUpdateInternetLocations", 0)
}

// NewSearcher creates a default searcher object
func NewSearcher(us *session.UpdateSession, criteria string, servers []string, thirdParty uint64) (*Searcher, error) {
	w, errors := wsus.Init(servers)
	if errors != nil {
		return nil, fmt.Errorf("Errors Initializing WSUS:\n%v", errors)
	}

	udi, err := us.CreateInterface(session.Searcher)
	if err != nil {
		return nil, err
	}

	serverSelection := w.ServerSelection
	serviceID := string(servicemgr.Default)
	if thirdParty == 1 && w.ServerSelection != wsus.ManagedServer {
		serverSelection = wsus.Others
		serviceID = string(servicemgr.MicrosoftUpdate)
	}

	return &Searcher{
		IUpdateSearcher: udi,
		Criteria:        criteria,
		ServerSelection: serverSelection,
		ServiceID:       serviceID,
	}, nil
}

// QueryUpdates uses the specified criteria to look up updates.
func (s *Searcher) QueryUpdates() (*updatecollection.Collection, error) {
	if err := s.configureRegistry(); err != nil {
		return nil, fmt.Errorf("failed to set registry values: %v", err)
	}

	// Set Update searcher properties
	if _, err := oleutil.PutProperty(s.IUpdateSearcher, "ServerSelection", s.ServerSelection); err != nil {
		return nil, fmt.Errorf("failed to set server selection property: \n %v", err)
	}

	// Set Update ServiceID
	if _, err := oleutil.PutProperty(s.IUpdateSearcher, "ServiceID", s.ServiceID); err != nil {
		return nil, fmt.Errorf("failed to set serviceID property: \n %v", err)
	}

	// Search for updates
	usr, err := oleutil.CallMethod(s.IUpdateSearcher, "Search", s.Criteria)
	if err != nil {
		s.SearchHResult = fmt.Sprintf("%s", errors.UpdateError(usr.Val))
		return nil, fmt.Errorf("search error: [%s] [%v]", s.SearchHResult, err)
	}
	s.SearchHResult = fmt.Sprintf("%s", errors.UpdateError(cablib.S_OK))
	s.ISearchResult = usr.ToIDispatch()

	// Get list of returned updates
	upd, err := oleutil.GetProperty(s.ISearchResult, "Updates")
	if err != nil {
		return nil, fmt.Errorf("error getting Updates collection, %s", err.Error())
	}

	// Save list to collection
	updd := updatecollection.Collection{IUpdateCollection: upd.ToIDispatch()}

	count, err := updd.Count()
	if err != nil {
		return nil, err
	}

	updd.Updates = make([]*updates.Update, count)
	for i := 0; i < count; i++ {
		item, err := oleutil.GetProperty(updd.IUpdateCollection, "item", i)
		if err != nil {
			return nil, err
		}
		itemd := item.ToIDispatch()

		up, errors := updates.New(itemd)
		if errors != nil {
			return nil, fmt.Errorf("errors in update enumeration: %v", errors)
		}
		updd.Updates[i] = up

	}
	return &updd, nil
}

// ResultCode gets an OperationResultCode enumeration that specifies the result of a search.
// Possible Result codes:
// 0 - (orcNotStarted)	The operation is not started.
// 1 - (orcInProgress)	The operation is in progress.
// 2 - (orcSucceeded)	The operation was completed successfully.
// 3 - (orcSucceededWithErrors)	The operation is complete, but one or more errors occurred during the operation. The results might be incomplete.
// 4 - (orcFailed)	The operation failed to complete.
// 5 - (orcAborted)	The operation is canceled.
func (s *Searcher) ResultCode() (int, error) {
	rc, err := oleutil.GetProperty(s.ISearchResult, "ResultCode")
	if err != nil {
		return 0, fmt.Errorf("error getting ResultCode property: %v", err)
	}
	return int(rc.Val), nil
}

// GetTotalHistoryCount returns the number of update events on the computer.
func (s *Searcher) GetTotalHistoryCount() (int, error) {
	c, err := oleutil.CallMethod(s.IUpdateSearcher, "GetTotalHistoryCount")
	if err != nil {
		return 0, fmt.Errorf("error getting update history count: %v", err)
	}

	return int(c.Val), nil
}

// QueryHistory synchronously queries the computer for the history of the update events.
func (s *Searcher) QueryHistory(count int) (*ole.IDispatch, error) {
	h, err := oleutil.CallMethod(s.IUpdateSearcher, "QueryHistory", 0, count)
	if err != nil {
		return nil, fmt.Errorf("error querying list of installed updates: %v", err)
	}
	return h.ToIDispatch(), nil
}

// Close releases objects created during search.
func (s *Searcher) Close() {
	s.IUpdateSearcher.Release()
	if s.ISearchResult != nil {
		s.ISearchResult.Release()
	}
}
