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

// Package search handles querying for Windows updates.
package search

import (
	"github.com/go-ole/go-ole"
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
