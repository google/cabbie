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

// Package updates expands an update item from IDispatch to a struct.
package updates

import (
	"time"

	"github.com/go-ole/go-ole"
)

// Identity represents the unique identifier of an update.
type Identity struct {
	RevisionNumber int
	UpdateID       string
}

// Category is information about a single Category.
type Category struct {
	Name       string
	Type       string
	CategoryID string
}

// Update contains the update interface and properties that are available to an update.
// See https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iupdate for details.
type Update struct {
	Item  *ole.IDispatch
	Title string

	AutoDownload             int
	AutoSelection            int
	BrowseOnly               bool
	CanRequireSource         bool
	Categories               []Category
	CveIDs                   []string
	Deadline                 time.Time
	Description              string
	EulaAccepted             bool
	Identity                 Identity
	IsBeta                   bool
	IsDownloaded             bool
	IsHidden                 bool
	IsInstalled              bool
	IsMandatory              bool
	IsPresent                bool
	IsUninstallable          bool
	KBArticleIDs             []string
	LastDeploymentChangeTime time.Time
	MaxDownloadSize          int
	MinDownloadSize          int
	MsrcSeverity             string
	PerUser                  bool
	RebootRequired           bool
	RecommendedCPUSpeed      int
	RecommendedHardDiskSpace int
	RecommendedMemory        int
	SecurityBulletinIDs      []string
	SupersededUpdateIDs      []string
	SupportURL               string
	Type                     string

	// If this is a driver, iUpdate is extended with the following IWindowsDriverUpdate properties.
	// See: https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iwindowsdriverupdate
	DeviceProblemNumber int
	DeviceStatus        int
	DriverClass         string
	DriverHardwareID    string
	DriverManufacturer  string
	DriverModel         string
	DriverProvider      string
	DriverVerDate       time.Time
}
