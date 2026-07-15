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

// FindUpdates queries the Windows Update Agent for available updates using the given criteria.
// If s is nil, FindUpdates creates a temporary UpdateSession and closes it when search finishes.
// If s is non-nil, FindUpdates reuses the provided session and leaves session cleanup to the caller.
// Returns the matching UpdateCollection, the search HResult metric string, and any error encountered.
func FindUpdates(s *session.UpdateSession, criteria string, servers []string, thirdParty uint64) (*updatecollection.Collection, string, error) {
	if s == nil {
		sess, err := session.New()
		if err != nil {
			return nil, "", fmt.Errorf("failed to create new Windows Update session: %w", err)
		}
		defer sess.Close()
		s = sess
	}

	q, err := NewSearcher(s, criteria, servers, thirdParty)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create searcher: %w", err)
	}
	defer q.Close()

	uc, err := q.QueryUpdates()
	if err != nil {
		return nil, q.SearchHResult, fmt.Errorf("error encountered when attempting to query for updates: %w", err)
	}

	return uc, q.SearchHResult, nil
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

func (s *Searcher) setupSearcherProperties() error {
	r, err := oleutil.PutProperty(s.IUpdateSearcher, "ServerSelection", s.ServerSelection)
	if r != nil {
		defer r.Clear()
	}
	if err != nil {
		return fmt.Errorf("failed to set server selection property: \n %v", err)
	}

	r2, err := oleutil.PutProperty(s.IUpdateSearcher, "ServiceID", s.ServiceID)
	if r2 != nil {
		defer r2.Clear()
	}
	if err != nil {
		return fmt.Errorf("failed to set serviceID property: \n %v", err)
	}
	return nil
}

func (s *Searcher) executeSearch() error {
	if s.ISearchResult != nil {
		s.ISearchResult.Release()
		s.ISearchResult = nil
	}

	usr, err := oleutil.CallMethod(s.IUpdateSearcher, "Search", s.Criteria)
	if usr != nil {
		defer usr.Clear()
	}
	if err != nil {
		if usr != nil {
			s.SearchHResult = fmt.Sprintf("%s", errors.UpdateError(usr.Val))
		}
		return fmt.Errorf("search error: [%s] [%v]", s.SearchHResult, err)
	}
	if usr == nil {
		return fmt.Errorf("search error: nil variant returned")
	}

	s.SearchHResult = fmt.Sprintf("%s", errors.UpdateError(cablib.S_OK))
	s.ISearchResult = usr.ToIDispatch()
	return nil
}

func (s *Searcher) extractCollection() (*updatecollection.Collection, error) {
	upd, err := oleutil.GetProperty(s.ISearchResult, "Updates")
	if upd != nil {
		defer upd.Clear()
	}
	if err != nil {
		return nil, fmt.Errorf("error getting Updates collection, %s", err.Error())
	}
	if upd == nil {
		return nil, fmt.Errorf("error getting Updates collection: nil variant returned")
	}

	updd := &updatecollection.Collection{IUpdateCollection: upd.ToIDispatch()}
	var errRet error
	defer func() {
		if errRet != nil {
			updd.Close()
		}
	}()

	count, err := updd.Count()
	if err != nil {
		errRet = err
		return nil, errRet
	}

	updd.Updates = make([]*updates.Update, count)
	for i := 0; i < count; i++ {
		item, err := oleutil.GetProperty(updd.IUpdateCollection, "item", i)
		if err != nil {
			errRet = err
			return nil, errRet
		}
		itemd := item.ToIDispatch()

		up, errors := updates.New(itemd)
		if errors != nil {
			itemd.Release()
			_ = item.Clear()
			errRet = fmt.Errorf("errors in update enumeration: %v", errors)
			return nil, errRet
		}
		updd.Updates[i] = up
		_ = item.Clear()
	}
	return updd, nil
}

// QueryUpdates uses the specified criteria to look up updates.
func (s *Searcher) QueryUpdates() (*updatecollection.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.configureRegistry(); err != nil {
		return nil, fmt.Errorf("failed to set registry values: %v", err)
	}

	if err := s.setupSearcherProperties(); err != nil {
		return nil, err
	}

	if err := s.executeSearch(); err != nil {
		return nil, err
	}

	return s.extractCollection()
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ISearchResult == nil {
		return 0, fmt.Errorf("ISearchResult is nil")
	}
	rc, err := oleutil.GetProperty(s.ISearchResult, "ResultCode")
	if rc != nil {
		defer rc.Clear()
	}
	if err != nil {
		return 0, fmt.Errorf("error getting ResultCode property: %v", err)
	}
	return int(rc.Val), nil
}

// GetTotalHistoryCount returns the number of update events on the computer.
func (s *Searcher) GetTotalHistoryCount() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.IUpdateSearcher == nil {
		return 0, fmt.Errorf("IUpdateSearcher is nil")
	}
	c, err := oleutil.CallMethod(s.IUpdateSearcher, "GetTotalHistoryCount")
	if c != nil {
		defer c.Clear()
	}
	if err != nil {
		return 0, fmt.Errorf("error getting update history count: %v", err)
	}

	return int(c.Val), nil
}

// QueryHistory synchronously queries the computer for the history of the update events.
func (s *Searcher) QueryHistory(count int) (*ole.IDispatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.IUpdateSearcher == nil {
		return nil, fmt.Errorf("IUpdateSearcher is nil")
	}
	h, err := oleutil.CallMethod(s.IUpdateSearcher, "QueryHistory", 0, count)
	if err != nil {
		if h != nil {
			_ = h.Clear()
		}
		return nil, fmt.Errorf("error querying list of installed updates: %v", err)
	}
	if h == nil {
		return nil, fmt.Errorf("error querying list of installed updates: nil variant returned")
	}
	return h.ToIDispatch(), nil
}

// Close releases objects created during search.
func (s *Searcher) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.IUpdateSearcher != nil {
		s.IUpdateSearcher.Release()
		s.IUpdateSearcher = nil
	}
	if s.ISearchResult != nil {
		s.ISearchResult.Release()
		s.ISearchResult = nil
	}
}
