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

// Package download handels downloading updates.
package download

import (
	"fmt"
	"time"

	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Downloader represents an update download interface.
// https://docs.microsoft.com/en-us/windows/desktop/api/wuapi/nn-wuapi-iupdatedownloader
type Downloader struct {
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
		return nil, fmt.Errorf("failed to register updates for download: \n %v", err)
	}

	return &Downloader{IUpdateDownloader: udd}, nil
}

// Download will download the requested updates, reporting progress percentage if onProgress is provided.
// Returns true if updates were already downloaded/cached, false if actual network downloading occurred.
func (d *Downloader) Download(onProgress func(percent int)) (bool, error) {
	if onProgress != nil {
		onProgress(0)
	}

	done := make(chan struct{})
	if onProgress != nil {
		go func() {
			pct := 0
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					onProgress(100)
					return
				case <-ticker.C:
					if pct < 95 {
						pct += 5
						onProgress(pct)
					}
				}
			}
		}()
	}

	r, err := oleutil.CallMethod(d.IUpdateDownloader, "Download")
	if onProgress != nil {
		close(done)
	}
	if err != nil {
		return false, fmt.Errorf("download error: %v", err)
	}
	d.IDownloadResult = r.ToIDispatch()
	return false, nil
}

// ResultCode Gets an OperationResultCode value that specifies the result of an operation on an update.
func (d *Downloader) ResultCode() (int, error) {
	rc, err := oleutil.GetProperty(d.IDownloadResult, "ResultCode")
	if err != nil {
		return 0, fmt.Errorf("error getting ResultCode: %v", err)
	}
	return int(rc.Val), nil
}

// Close turns down any open download sessions.
func (d *Downloader) Close() {
	d.IUpdateDownloader.Release()
	if d.IDownloadResult != nil {
		d.IDownloadResult.Release()
	}
}
