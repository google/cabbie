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

// Package updatecollection manages an ordered list of updates.
package updatecollection

import (
	"fmt"

	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/updates"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Collection represents an ordered list of updates.
type Collection struct {
	IUpdateCollection *ole.IDispatch
	Updates           []*updates.Update
}

// New creates an empty update collection.
func New() (*Collection, error) {
	uc, err := cablib.NewCOMObject("Microsoft.Update.UpdateColl")
	if err != nil {
		return nil, err
	}
	return &Collection{IUpdateCollection: uc}, nil

}

// Count gets the number of updates in an UpdateCollection.
func (uc *Collection) Count() (int, error) {
	count, err := oleutil.GetProperty(uc.IUpdateCollection, "Count")
	if err != nil {
		return 0, fmt.Errorf("error getting update collection count, %v", err)
	}
	defer count.Clear()
	return int(count.Val), nil
}

// Add adds an update item to the collection.
func (uc *Collection) Add(item *ole.IDispatch) error {
	if _, err := oleutil.CallMethod(uc.IUpdateCollection, "Add", item); err != nil {
		return fmt.Errorf("error adding to collection, %v", err)
	}
	return nil
}

// Clear removes all the update items from the collection.
func (uc *Collection) Clear() error {
	if _, err := oleutil.CallMethod(uc.IUpdateCollection, "Clear"); err != nil {
		return fmt.Errorf("error clearing collection, %v", err)
	}
	return nil
}

// Refresh ensures Collection.Updates data is current.
func (uc *Collection) Refresh() error {
	count, err := uc.Count()
	if err != nil {
		return err
	}
	uc.closeItems()

	uc.Updates = make([]*updates.Update, count)
	for i := 0; i < count; i++ {
		item, err := oleutil.GetProperty(uc.IUpdateCollection, "item", i)
		if err != nil {
			return err
		}
		itemd := item.ToIDispatch()

		up, errors := updates.New(itemd)
		if len(errors) > 0 {
			return fmt.Errorf("errors in update enumeration: %v", errors)
		}
		uc.Updates[i] = up
	}
	return nil
}

// Titles returns a slice of update titles.
func (uc *Collection) Titles() []string {
	var t []string
	for _, u := range uc.Updates {
		t = append(t, u.Title)
	}
	return t
}

// Close turns down any open update sessions.
func (uc *Collection) Close() {
	uc.IUpdateCollection.Release()
	uc.closeItems()
}

func (uc *Collection) closeItems() {
	for i := 0; i < len(uc.Updates); i++ {
		uc.Updates[i].Item.Release()
	}
}
