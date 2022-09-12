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

// Package updatehistory expands an updatehistory item from IDispatch to a struct.
package updatehistory

import (
	"time"

	"github.com/google/cabbie/updates"
	"github.com/go-ole/go-ole"
)

// History represents an ordered read-only list of IUpdateHistoryEntry interfaces.
type History struct {
	IUpdateHistoryEntryCollection *ole.IDispatch
	Entries                       []*Entry
}

// Entry represents the recorded history of an update.
type Entry struct {
	Item                *ole.IDispatch
	Operation           int
	ResultCode          int
	HResult             int
	Date                time.Time
	UpdateIdentity      updates.Identity
	Title               string
	Description         string
	UnmappedResultCode  int
	ClientApplicationID string
	ServerSelection     int
	ServiceID           string
	UninstallationNotes string
	SupportURL          string
	Categories          []updates.Category
}
