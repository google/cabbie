// Copyright 2020 Google LLC
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

package updates

import (
	"reflect"
	"testing"
)

var (
	u = Update{
		Title: "Title string",
		Categories: []Category{
			Category{
				Name: "foo",
			},
		},
	}
)

func TestInCategories(t *testing.T) {
	for _, tt := range []struct {
		in  []string
		out bool
	}{
		{[]string{"foo"}, true},
		{[]string{"bar", "foo"}, true},
		{[]string{"baz"}, false},
	} {
		o := u.InCategories(tt.in)
		if !reflect.DeepEqual(o, tt.out) {
			t.Errorf("InCategories(%v) = %v, want %v", tt.in, o, tt.out)
		}
	}
}

func TestFillStruct(t *testing.T) {
	data := make(map[string]interface{})
	for _, tt := range []struct {
		p     string
		d     interface{}
		isNil bool
	}{
		{"MinDownloadSize", 100, true},
		{"Type", "Definition Updates", true},
		{"AutoDownload", "1", false},
		{"SecurityBulletinIDs", []string{"1000", "2000", "12342314"}, false},
	} {
		data[tt.p] = tt.d
		err := u.fillStruct(data)
		if (err == nil) != tt.isNil {
			t.Errorf("Verify Nil Error: fillStruct(%v) = %v, wanted nil: %v", data, err, tt.isNil)
		}
	}
}
