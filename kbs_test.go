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
package main

import (
	"testing"
)

func TestSearch(t *testing.T) {
	for _, tt := range []struct {
		in    KBSet
		query []string
		out   bool
	}{
		{NewKBSet(""), []string{"KB123456"}, false},
		{NewKBSet("KB987654,KB123456"), []string{"KB123456"}, true},
		{NewKBSet("KB987654"), []string{"KB987654"}, true},
		{NewKBSet("KB987654,KB123456"), []string{"KB654321"}, false},
		{NewKBSet("KB987654,KB123456"), []string{""}, false},
		{NewKBSet("KB987654"), []string{"kb987654"}, true},
		{NewKBSet("123456"), []string{"KB123456"}, true},
		{NewKBSet("KB654321"), []string{"654321"}, true},
	} {
		o := tt.in.Search(tt.query)
		if o != tt.out {
			t.Errorf("Search(%s, %+v): got %t, want %t", tt.in, tt.query, o, tt.out)
		}
	}
}

func TestSize(t *testing.T) {
	for _, tt := range []struct {
		in  KBSet
		out int
	}{
		{NewKBSet(""), 0},
		{NewKBSet("KB123456"), 1},
		{NewKBSet("KB123456,KB987654"), 2},
	} {
		o := tt.in.Size()
		if o != tt.out {
			t.Errorf("got %d, want %d", o, tt.out)
		}
	}
}

func TestString(t *testing.T) {
	for _, tt := range []struct {
		in  KBSet
		out string
	}{
		{NewKBSet(""), "[]"},
		{NewKBSet("KB123456"), "[KB123456]"},
		{NewKBSet("KB123456,KB987654"), "[KB123456 KB987654]"},
	} {
		o := tt.in.String()
		if o != tt.out {
			t.Errorf("got %s, want %s", o, tt.out)
		}
	}
}
