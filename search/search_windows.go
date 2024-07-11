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
	"github.com/google/deck"
	"golang.org/x/sys/windows/registry"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

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
			deck.Errorf("errors in update enumeration: %v", errors)
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
