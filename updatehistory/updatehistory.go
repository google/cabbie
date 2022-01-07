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

// Package updatehistory expands an updatehistory item from IDispatch to a struct.
package updatehistory

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/updates"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
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

// New expands an IUpdateHistoryEntry object into a usable go struct
func New(item *ole.IDispatch) (*Entry, error) {
	e := &Entry{Item: item}

	fields := reflect.TypeOf(*e)
	data := make(map[string]interface{})
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		p := field.Name
		switch field.Type.String() {
		case "string":
			data[p], _ = e.toString(p)
		case "int":
			data[p], _ = e.toInt(p)
		case "time.Time":
			data[p], _ = e.toDateTime(p)
		case "[]updates.Category":
			data[p], _ = e.toCategories(p)
		case "updates.Identity":
			data[p], _ = e.toIdentity(p)
		}
	}

	if err := e.fillStruct(data); err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Entry) toString(property string) (string, error) {
	p, err := oleutil.GetProperty(e.Item, property)
	if err != nil {
		return "", err
	}
	return p.ToString(), nil
}

func (e *Entry) toInt(property string) (int, error) {
	p, err := oleutil.GetProperty(e.Item, property)
	if err != nil {
		return 0, nil
	}

	if p.Value() == nil {
		return 0, nil
	}
	return int(p.Value().(int32)), nil
}

func (e *Entry) toDateTime(property string) (time.Time, error) {
	p, err := oleutil.GetProperty(e.Item, property)
	if err != nil {
		return time.Time{}, err
	}

	if p.Value() == nil {
		return time.Time{}, nil
	}
	return p.Value().(time.Time), nil
}

func (e *Entry) toIdentity(property string) (updates.Identity, error) {
	i := updates.Identity{}
	p, err := oleutil.GetProperty(e.Item, property)
	if err != nil {
		return updates.Identity{}, err
	}
	pd := p.ToIDispatch()
	defer pd.Release()

	rn, err := oleutil.GetProperty(pd, "RevisionNumber")
	if err != nil {
		return updates.Identity{}, err
	}
	i.RevisionNumber = int(rn.Value().(int32))

	uid, err := oleutil.GetProperty(pd, "UpdateID")
	if err != nil {
		return updates.Identity{}, err
	}
	i.UpdateID = uid.ToString()

	return i, nil
}

func (e *Entry) toCategories(property string) ([]updates.Category, error) {
	cs := []updates.Category{}
	cats, err := oleutil.GetProperty(e.Item, "Categories")
	if err != nil {
		return cs, err
	}
	catsd := cats.ToIDispatch()
	defer catsd.Release()

	count, err := cablib.Count(catsd)
	if err != nil {
		return cs, err
	}

	for i := 0; i < count; i++ {
		item, err := oleutil.GetProperty(catsd, "item", i)
		if err != nil {
			continue
		}
		itemd := item.ToIDispatch()

		n, err := oleutil.GetProperty(itemd, "Name")
		if err != nil {
			itemd.Release()
			continue
		}
		t, err := oleutil.GetProperty(itemd, "Type")
		if err != nil {
			n.Clear()
			itemd.Release()
			continue
		}
		c, err := oleutil.GetProperty(itemd, "CategoryID")
		if err != nil {
			n.Clear()
			t.Clear()
			itemd.Release()
			continue
		}

		cs = append(cs, updates.Category{
			Name:       n.ToString(),
			Type:       t.ToString(),
			CategoryID: c.ToString()})
		itemd.Release()
		n.Clear()
		t.Clear()
		c.Clear()
	}

	return cs, nil
}

func (e *Entry) fillStruct(m map[string]interface{}) error {
	for k, v := range m {
		if err := cablib.SetField(e, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (e *Entry) String() string {
	return fmt.Sprintf("Title: %s\n"+
		"UpdateIdentity: %+v\n"+
		"ClientApplicationID: %s\n"+
		"SupportURL: %s\n"+
		"Categories: %+v", e.Title, e.UpdateIdentity, e.ClientApplicationID, e.SupportURL, e.Categories)
}

// Get returns a history object containing the list of update history entries.
func Get(searchInterface *search.Searcher) (*History, error) {
	c, err := searchInterface.GetTotalHistoryCount()
	if err != nil {
		return nil, err
	}

	hc, err := searchInterface.QueryHistory(c)
	if err != nil {
		return nil, err
	}

	h := History{IUpdateHistoryEntryCollection: hc}

	count, err := h.Count()
	if err != nil {
		h.Close()
		return nil, err
	}

	h.Entries = make([]*Entry, 0)
	for i := 0; i < count; i++ {
		item, err := oleutil.GetProperty(h.IUpdateHistoryEntryCollection, "item", i)
		if err != nil {
			h.Close()
			return nil, err
		}
		itemd := item.ToIDispatch()

		uh, err := New(itemd)
		if err != nil {
			itemd.Release()
			h.Close()
			return nil, fmt.Errorf("errors in update enumeration: %v", err)
		}

		// Weed out random invalid entries that show up for some reason.
		if uh.Operation != 0 {
			h.Entries = append(h.Entries, uh)
		}
	}

	return &h, nil
}

// Count gets the number of updates in an IUpdateHistoryEntryCollection.
func (hc *History) Count() (int, error) {
	count, err := oleutil.GetProperty(hc.IUpdateHistoryEntryCollection, "Count")
	if err != nil {
		return 0, fmt.Errorf("error getting history collection count, %v", err)
	}
	defer count.Clear()
	return int(count.Val), nil
}

// Close turns down any open update sessions.
func (hc *History) Close() {
	hc.IUpdateHistoryEntryCollection.Release()
	hc.closeItems()
}

func (hc *History) closeItems() {
	//TODO(b/136258504) Using range causes application to occasionally hang.
	for i := 0; i < len(hc.Entries); i++ {
		if hc.Entries[i] != nil {
			hc.Entries[i].Item.Release()
		}
	}
}
