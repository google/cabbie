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

package updatehistory

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/updates"
)

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
	out := p.ToString()
	_ = p.Clear()
	return out, nil
}

func (e *Entry) toInt(property string) (int, error) {
	var out int
	p, err := oleutil.GetProperty(e.Item, property)
	if err != nil {
		return 0, nil
	}
	if p.Value() != nil {
		out = int(p.Value().(int32))
	}
	_ = p.Clear()
	return out, nil
}

func (e *Entry) toDateTime(property string) (time.Time, error) {
	var out time.Time
	p, err := oleutil.GetProperty(e.Item, property)
	if err != nil {
		return time.Time{}, err
	}
	if p.Value() != nil {
		out = p.Value().(time.Time)
	}
	_ = p.Clear()
	return out, nil
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
	_ = rn.Clear()

	uid, err := oleutil.GetProperty(pd, "UpdateID")
	if err != nil {
		return updates.Identity{}, err
	}
	i.UpdateID = uid.ToString()
	_ = uid.Clear()

	return i, nil
}

func (e *Entry) toCategories(string) ([]updates.Category, error) {
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
			_ = n.Clear()
			itemd.Release()
			continue
		}
		c, err := oleutil.GetProperty(itemd, "CategoryID")
		if err != nil {
			_ = n.Clear()
			_ = t.Clear()
			itemd.Release()
			continue
		}

		cs = append(cs, updates.Category{
			Name:       n.ToString(),
			Type:       t.ToString(),
			CategoryID: c.ToString()})
		itemd.Release()
		_ = n.Clear()
		_ = t.Clear()
		_ = c.Clear()
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
		"Categories: %+v\n"+
		"Date: %s", e.Title, e.UpdateIdentity, e.ClientApplicationID, e.SupportURL, e.Categories, e.Date)
}

// Get returns:
// History object containing the list of update history entries
// count of update events
// error
func Get(searchInterface *search.Searcher) (*History, int, error) {
	updateHistoryCount, err := searchInterface.GetTotalHistoryCount()
	if err != nil {
		return nil, updateHistoryCount, err
	}
	if updateHistoryCount == 0 {
		return nil, updateHistoryCount, nil
	}

	hc, err := searchInterface.QueryHistory(updateHistoryCount)
	if err != nil {
		return nil, updateHistoryCount, err
	}

	h := History{IUpdateHistoryEntryCollection: hc}

	count, err := h.Count()
	if err != nil {
		h.Close()
		return nil, updateHistoryCount, err
	}

	for i := 0; i < count; i++ {
		item, err := oleutil.GetProperty(h.IUpdateHistoryEntryCollection, "item", i)
		if err != nil {
			h.Close()
			return nil, updateHistoryCount, err
		}
		itemd := item.ToIDispatch()
		uh, err := New(itemd)
		if err != nil {
			itemd.Release()
			h.Close()
			return nil, updateHistoryCount, fmt.Errorf("errors in update enumeration: %v", err)
		}
		// Weed out random invalid entries that show up for some reason.
		if uh.Operation != 0 {
			h.Entries = append(h.Entries, uh)
		}
		_ = item.Clear()
	}

	return &h, updateHistoryCount, nil
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
}
