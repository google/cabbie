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

// Package download handles downloading updates.
package download

import (
	"golang.org/x/net/context"
	"fmt"
	"sync"

	"github.com/google/cabbie/errors"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Downloader represents an update download interface.
// https://docs.microsoft.com/en-us/windows/desktop/api/wuapi/nn-wuapi-iupdatedownloader
type Downloader struct {
	mu                sync.Mutex
	IUpdateDownloader *ole.IDispatch
	IDownloadResult   *ole.IDispatch
}

// NewDownloader creates an update download interface with a specified update collection.
func NewDownloader(us *session.UpdateSession, uc *updatecollection.Collection) (*Downloader, error) {
	udd, err := us.CreateInterface(session.Downloader)
	if err != nil {
		return nil, err
	}

	if _, err = oleutil.PutProperty(udd, "Updates", uc.IUpdateCollection); err != nil {
		udd.Release()
		return nil, fmt.Errorf("failed to register updates for download: \n %v", err)
	}

	return &Downloader{IUpdateDownloader: udd}, nil
}

// Download will download the requested updates.
func (d *Downloader) Download(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.IUpdateDownloader == nil {
		return fmt.Errorf("download error: nil IUpdateDownloader dispatch")
	}
	r, err := oleutil.CallMethod(d.IUpdateDownloader, "Download")
	if err != nil {
		var val int64
		if r != nil {
			val = r.Val
			r.Clear()
		}
		return fmt.Errorf("download error: [%s] [%v]", errors.UpdateError(val), err)
	}
	if r == nil {
		return fmt.Errorf("download error: nil variant returned")
	}
	d.IDownloadResult = r.ToIDispatch()
	return nil
}

// ResultCode Gets an OperationResultCode value that specifies the result of an operation on an update.
func (d *Downloader) ResultCode() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.IDownloadResult == nil {
		return 0, fmt.Errorf("error getting ResultCode: nil IDownloadResult dispatch")
	}
	rc, err := oleutil.GetProperty(d.IDownloadResult, "ResultCode")
	if err != nil {
		if rc != nil {
			rc.Clear()
		}
		return 0, fmt.Errorf("error getting ResultCode: %v", err)
	}
	if rc == nil {
		return 0, fmt.Errorf("error getting ResultCode: nil variant returned")
	}
	defer rc.Clear()
	return int(rc.Val), nil
}

// Close turns down any open download sessions.
func (d *Downloader) Close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.IUpdateDownloader != nil {
		d.IUpdateDownloader.Release()
		d.IUpdateDownloader = nil
	}
	if d.IDownloadResult != nil {
		d.IDownloadResult.Release()
		d.IDownloadResult = nil
	}
}

