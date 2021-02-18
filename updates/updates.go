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
	"fmt"
	"reflect"
	"time"

	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/errors"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
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
type Update struct {
	Item                     *ole.IDispatch
	Title                    string
	CanRequireSource         bool
	Categories               []Category
	Deadline                 time.Time
	Description              string
	EulaAccepted             bool
	Identity                 Identity
	IsBeta                   bool
	IsDownloaded             bool
	IsHidden                 bool
	IsInstalled              bool
	IsMandatory              bool
	IsUninstallable          bool
	LastDeploymentChangeTime time.Time
	MaxDownloadSize          int
	MinDownloadSize          int
	MsrcSeverity             string
	RecommendedCPUSpeed      int
	RecommendedHardDiskSpace int
	RecommendedMemory        int
	SecurityBulletinIDs      []string
	SupersededUpdateIDs      []string
	SupportURL               string
	Type                     string
	KBArticleIDs             []string
	RebootRequired           bool
	IsPresent                bool
	CveIDs                   []string
	BrowseOnly               bool
	PerUser                  bool
	AutoSelection            int
	AutoDownload             int
}

// New expands an IUpdate object into a usable go struct.
func New(item *ole.IDispatch) (*Update, []error) {
	var errors []error
	u := &Update{Item: item}

	fields := reflect.TypeOf(*u)
	data := make(map[string]interface{})
	var err error
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		p := field.Name
		switch field.Type.String() {
		case "string":
			data[p], err = u.toString(p)
			if err != nil {
				errors = append(errors, err)
			}
		case "bool":
			data[p], err = u.toBool(p)
			if err != nil {
				errors = append(errors, err)
			}
		case "int":
			data[p], err = u.toInt(p)
			if err != nil {
				errors = append(errors, err)
			}
		case "[]string":
			data[p], err = u.toStringSlice(p)
			if err != nil {
				errors = append(errors, err)
			}
		case "time.Time":
			data[p], err = u.toDateTime(p)
			if err != nil {
				errors = append(errors, err)
			}
		case "[]updates.Category":
			data[p], err = u.toCategories(p)
			if err != nil {
				errors = append(errors, err)
			}
		case "updates.Identity":
			data[p], err = u.toIdentity(p)
			if err != nil {
				errors = append(errors, err)
			}
		}
	}

	if err := u.fillStruct(data); err != nil {
		errors = append(errors, err)
	}

	return u, errors
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

func (up *Update) toString(property string) (string, error) {
	p, err := oleutil.GetProperty(up.Item, property)
	if err != nil {
		return "", err
	}
	return p.ToString(), nil
}

func (up *Update) toBool(property string) (bool, error) {
	p, err := oleutil.GetProperty(up.Item, property)
	if err != nil {
		return false, err
	}
	return p.Value().(bool), nil
}

func (up *Update) toInt(property string) (int, error) {
	p, err := oleutil.GetProperty(up.Item, property)
	if err != nil {
		return 0, err
	}

	if p.Value() == nil {
		return 0, nil
	}
	return int(p.Value().(int32)), nil
}

func (up *Update) toDateTime(property string) (time.Time, error) {
	p, err := oleutil.GetProperty(up.Item, property)
	if err != nil {
		return time.Time{}, err
	}

	if p.Value() == nil {
		return time.Time{}, nil
	}
	return p.Value().(time.Time), nil
}

func (up *Update) toStringSlice(property string) ([]string, error) {
	p, err := oleutil.GetProperty(up.Item, property)
	if err != nil {
		return nil, err
	}
	pd := p.ToIDispatch()
	defer pd.Release()

	count, err := cablib.Count(pd)
	if err != nil {
		return nil, err
	}

	r := make([]string, count)
	for i := 0; i < count; i++ {
		prop, err := oleutil.GetProperty(pd, "Item", i)
		if err != nil {
			return nil, err
		}
		r[i] = prop.ToString()
	}
	return r, nil
}

func (up *Update) toCategories(property string) ([]Category, error) {
	cs := []Category{}
	cats, err := oleutil.GetProperty(up.Item, "Categories")
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
			itemd.Release()
			continue
		}
		c, err := oleutil.GetProperty(itemd, "CategoryID")
		if err != nil {
			itemd.Release()
			continue
		}

		cs = append(cs, Category{
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

func (up *Update) toIdentity(property string) (Identity, error) {
	p, err := oleutil.GetProperty(up.Item, property)
	if err != nil {
		return Identity{}, err
	}
	pd := p.ToIDispatch()
	defer pd.Release()

	rn, err := oleutil.GetProperty(pd, "RevisionNumber")
	if err != nil {
		return Identity{}, err
	}
	uid, err := oleutil.GetProperty(pd, "UpdateID")
	if err != nil {
		return Identity{}, err
	}

	return Identity{RevisionNumber: int(rn.Value().(int32)),
		UpdateID: uid.ToString()}, nil
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
