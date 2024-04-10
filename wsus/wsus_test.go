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

package wsus

import (
	"reflect"
	"testing"
)

func TestSortedKeys(t *testing.T) {
	for _, tt := range []struct {
		in  map[int]string
		out []int
	}{
		{map[int]string{2: "foo", 1: "bar", 3: "baz"}, []int{1, 2, 3}},
		{map[int]string{1: "foo", 2: "bar", 3: "baz"}, []int{1, 2, 3}},
	} {
		o := sortedKeys(tt.in)
		if !reflect.DeepEqual(o, tt.out) {
			t.Errorf("SortedKeys(%v) = %v, want %v", tt.in, o, tt.out)
		}
	}
}

func TestSet(t *testing.T) {
	w := WSUS{
		CurrentServer:   "",
		ServerSelection: 0,
		Servers:         []string{"foo.biz", "bar.biz"},
	}

	for _, tt := range []struct {
		in            int
		currentServer string
		isNil         bool
	}{
		{2, "", false},
		{1, "bar.biz", true},
		{0, "foo.biz", true},
	} {
		err := w.Set(tt.in)
		if (err == nil) != tt.isNil {
			t.Errorf("Verify Nil Error:\nSet(%v) = %v, want %v", tt.in, err, tt.isNil)
		}
		if w.CurrentServer != tt.currentServer {
			t.Errorf("Verify Current Server:\nSet(%v) = %v, want %v", tt.in, w.CurrentServer, tt.currentServer)
		}
	}
}
