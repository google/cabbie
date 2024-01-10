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

package updates

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/errors"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// New expands an IUpdate object into a usable go struct.
func New(item *ole.IDispatch) (*Update, []error) {
	var errs []error
	u := &Update{Item: item}

	fields := reflect.TypeOf(*u)
	data := make(map[string]interface{})
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		p := field.Name
		if p == "Item" {
			continue
		}

		driverProperties := map[string]bool{
			"DeviceProblemNumber": true,
			"DeviceStatus":        true,
			"DriverClass":         true,
			"DriverHardwareID":    true,
			"DriverManufacturer":  true,
			"DriverModel":         true,
			"DriverProvider":      true,
			"DriverVerDate":       true,
		}
		if driverProperties[p] {
			// Check if IUpdate object also implements any IWindowsDriverUpdate property.
			// If not, skip attempting to extract IWindowsDriverUpdate properties.
			// See https://docs.microsoft.com/en-us/windows/win32/api/wuapi/nn-wuapi-iwindowsdriverupdate
			if _, err := u.Item.QueryInterface(cablib.IIDIWindowsDriverUpdate); err != nil {
				continue
			}
		}

		v, err := oleutil.GetProperty(u.Item, p)
		if err != nil {
			errs = append(errs, fmt.Errorf("get property %q: %w", p, err))
			continue
		}

		switch field.Type.String() {
		case "bool":
			data[p] = toBool(v)
		case "int":
			data[p] = toInt(v)
		case "string":
			data[p] = toString(v)
		case "time.Time":
			data[p] = toTime(v)
		case "[]string":
			data[p], err = toStringSlice(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("extract string slice for %q: %w", p, err))
			}
		case "[]updates.Category":
			data[p], err = toCategories(v)
			if err != nil {
				errs = append(errs, fmt.Errorf("extract category slice: %w", err))
			}
		case "updates.Identity":
			data[p], err = toIdentity(v)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	if err := u.fillStruct(data); err != nil {
		errs = append(errs, err)
	}

	return u, errs
}

// AcceptEula accepts the Microsoft Software License Terms that are associated with Windows Update.
func (up *Update) AcceptEula() error {
	r, err := oleutil.CallMethod(up.Item, "AcceptEula")
	if err != nil {
		return fmt.Errorf("unable to accept Eula: [%s] [%v]", errors.UpdateError(r.Val), err)
	}
	up.EulaAccepted = true
	return nil
}

// Hide sets a Boolean value that hides the update from future search results.
func (up *Update) Hide() error {
	r, err := oleutil.PutProperty(up.Item, "IsHidden", true)
	if err != nil {
		return fmt.Errorf("unable to hide update: [%s] [%v]", errors.UpdateError(r.Val), err)
	}
	up.IsHidden = true
	return nil
}

// UnHide sets a Boolean value that makes the update available in future search results.
func (up *Update) UnHide() error {
	r, err := oleutil.PutProperty(up.Item, "IsHidden", false)
	if err != nil {
		return fmt.Errorf("failed to unhide update: [%s] [%v]", errors.UpdateError(r.Val), err)
	}
	up.IsHidden = false
	return nil
}

func toString(v *ole.VARIANT) string {
	return v.ToString()
}

func toBool(v *ole.VARIANT) bool {
	return v.Value().(bool)
}

func toInt(v *ole.VARIANT) int {
	if v.Value() == nil {
		return 0
	}
	return int(v.Value().(int32))
}

func toTime(v *ole.VARIANT) time.Time {
	if v.Value() == nil {
		return time.Time{}
	}
	return v.Value().(time.Time)
}

func forEachIn(v *ole.VARIANT, do func(item *ole.VARIANT) error) error {
	pd := v.ToIDispatch()
	defer pd.Release()

	count, err := cablib.Count(pd)
	if err != nil {
		return fmt.Errorf("count: %w", err)
	}

	for i := 0; i < count; i++ {
		item, err := oleutil.GetProperty(pd, "Item", i)
		if err != nil {
			return fmt.Errorf("get item %d: %w", i, err)
		}

		if err := do(item); err != nil {
			return fmt.Errorf("do item %d: %w", i, err)
		}
	}
	return nil
}

func toStringSlice(v *ole.VARIANT) ([]string, error) {
	var r []string
	if err := forEachIn(v, func(item *ole.VARIANT) error {
		r = append(r, toString(item))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("looping strings: %w", err)
	}
	return r, nil
}

func toCategories(v *ole.VARIANT) ([]Category, error) {
	var r []Category
	if err := forEachIn(v, func(item *ole.VARIANT) error {
		itemd := item.ToIDispatch()
		defer itemd.Release()

		n, err := oleutil.GetProperty(itemd, "Name")
		if err != nil {
			return fmt.Errorf("get category name: %w", err)
		}
		defer n.Clear()

		t, err := oleutil.GetProperty(itemd, "Type")
		if err != nil {
			return fmt.Errorf("get category type: %w", err)
		}
		defer t.Clear()

		c, err := oleutil.GetProperty(itemd, "CategoryID")
		if err != nil {
			return fmt.Errorf("get category id: %w", err)
		}
		defer c.Clear()

		r = append(r, Category{
			Name:       toString(n),
			Type:       toString(t),
			CategoryID: toString(c),
		})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("looping categories: %w", err)
	}
	return r, nil
}

func toIdentity(v *ole.VARIANT) (Identity, error) {
	pd := v.ToIDispatch()
	defer pd.Release()

	rn, err := oleutil.GetProperty(pd, "RevisionNumber")
	if err != nil {
		return Identity{}, err
	}
	defer rn.Clear()

	uid, err := oleutil.GetProperty(pd, "UpdateID")
	if err != nil {
		return Identity{}, err
	}
	defer uid.Clear()

	return Identity{
		RevisionNumber: toInt(rn),
		UpdateID:       toString(uid),
	}, nil
}

func (up *Update) String() string {
	return fmt.Sprintf("Title: %s\n"+
		"Categories: %+v\n"+
		"MsrcSeverity: %s\n"+
		"EulaAccepted: %t\n"+
		"KBArticleIDs: %v", up.Title, up.Categories, up.MsrcSeverity, up.EulaAccepted, up.KBArticleIDs)
}

func (up *Update) fillStruct(m map[string]interface{}) error {
	for k, v := range m {
		if err := cablib.SetField(up, k, v); err != nil {
			return err
		}
	}
	return nil
}

// InCategories determines whether or not this update is in one of the supplied categories.
func (up *Update) InCategories(categories []string) bool {
	if len(categories) == 0 {
		return true
	}

	for _, v := range up.Categories {
		if cablib.StringInSlice(v.Name, categories) {
			return true
		}
	}
	return false
}
