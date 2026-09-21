// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"testing"

	"github.com/google/cabbie/updates"
)

const (
	testUpdateID  = "8870bdb3-95f9-43d9-9ba4-1f0e7cf13db2"
	otherUpdateID = "245bd515-7b95-496e-acac-344881833263"
)

// cumulative is a typical update carrying both a KB article ID and an update
// ID, so it can be targeted by either identifier.
func cumulative() *updates.Update {
	return &updates.Update{
		Title:        "Cumulative Update for Windows",
		KBArticleIDs: []string{"5034441"},
		Identity:     updates.Identity{UpdateID: testUpdateID},
	}
}

func TestUpdateSetEmpty(t *testing.T) {
	for _, tt := range []struct {
		desc string
		in   updateSet
		want bool
	}{
		{"zero value", updateSet{}, true},
		{"empty kb list", updateSet{kbs: NewKBSet("")}, true},
		{"nil kb slice and nil uuids", updateSet{kbs: NewKBSetFromSlice(nil), uuids: nil}, true},
		{"empty uuid slice", updateSet{uuids: []string{}}, true},
		{"kbs only", updateSet{kbs: NewKBSet("KB5034441")}, false},
		{"uuids only", updateSet{uuids: []string{testUpdateID}}, false},
		{"both", updateSet{kbs: NewKBSet("KB5034441"), uuids: []string{testUpdateID}}, false},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			if got := tt.in.empty(); got != tt.want {
				t.Errorf("updateSet.empty() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestUpdateSetMatches(t *testing.T) {
	// A driver update has no KB article ID, so it is only addressable by update ID.
	driver := &updates.Update{
		Title:    "Intel - Net",
		Identity: updates.Identity{UpdateID: testUpdateID},
	}
	multiKB := &updates.Update{
		Title:        "Update with several KBs",
		KBArticleIDs: []string{"5049296", "5034441"},
		Identity:     updates.Identity{UpdateID: testUpdateID},
	}

	for _, tt := range []struct {
		desc string
		in   updateSet
		u    *updates.Update
		want bool
	}{
		{"empty set matches nothing", updateSet{}, cumulative(), false},
		{"kb match", updateSet{kbs: NewKBSet("5034441")}, cumulative(), true},
		{"kb match with prefix in set", updateSet{kbs: NewKBSet("KB5034441")}, cumulative(), true},
		{"kb match with prefix on update", updateSet{kbs: NewKBSet("5034441")}, &updates.Update{KBArticleIDs: []string{"KB5034441"}}, true},
		{"kb mismatch", updateSet{kbs: NewKBSet("5049296")}, cumulative(), false},
		{"kb match against one of several", updateSet{kbs: NewKBSet("5034441")}, multiKB, true},
		{"kb set against update with no kbs", updateSet{kbs: NewKBSet("5034441")}, driver, false},
		{"update id match", updateSet{uuids: []string{testUpdateID}}, cumulative(), true},
		{"update id match is case insensitive", updateSet{uuids: []string{"8870BDB3-95F9-43D9-9BA4-1F0E7CF13DB2"}}, cumulative(), true},
		{"update id match against update with no kbs", updateSet{uuids: []string{testUpdateID}}, driver, true},
		{"update id mismatch", updateSet{uuids: []string{otherUpdateID}}, cumulative(), false},
		{"matches on update id when kb does not match", updateSet{kbs: NewKBSet("5049296"), uuids: []string{testUpdateID}}, cumulative(), true},
		{"matches on kb when update id does not match", updateSet{kbs: NewKBSet("5034441"), uuids: []string{otherUpdateID}}, cumulative(), true},
		{"neither identifier matches", updateSet{kbs: NewKBSet("5049296"), uuids: []string{otherUpdateID}}, cumulative(), false},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			if got := tt.in.matches(tt.u); got != tt.want {
				t.Errorf("updateSet.matches(%q) = %t, want %t", tt.u.Title, got, tt.want)
			}
		})
	}
}

func TestDecideUnhide(t *testing.T) {
	for _, tt := range []struct {
		desc   string
		want   updateSet
		hidden updateSet
		out    unhideAction
	}{
		{
			"update was not requested to be unhidden",
			updateSet{kbs: NewKBSet("5049296")},
			updateSet{},
			skipUnrelated,
		},
		{
			"requested by kb",
			updateSet{kbs: NewKBSet("5034441")},
			updateSet{},
			applyUnhide,
		},
		{
			"requested by update id",
			updateSet{uuids: []string{testUpdateID}},
			updateSet{},
			applyUnhide,
		},
		{
			"hidden by the same kb",
			updateSet{kbs: NewKBSet("5034441")},
			updateSet{kbs: NewKBSet("5034441")},
			skipConflict,
		},
		{
			"hidden by the same update id",
			updateSet{uuids: []string{testUpdateID}},
			updateSet{uuids: []string{testUpdateID}},
			skipConflict,
		},
		{
			// reconcile cannot catch this, as it only compares configured strings.
			"hidden by kb, requested to be unhidden by update id",
			updateSet{uuids: []string{testUpdateID}},
			updateSet{kbs: NewKBSet("5034441")},
			skipConflict,
		},
		{
			"hidden by update id, requested to be unhidden by kb",
			updateSet{kbs: NewKBSet("5034441")},
			updateSet{uuids: []string{testUpdateID}},
			skipConflict,
		},
		{
			"hidden set covers only other updates",
			updateSet{kbs: NewKBSet("5034441")},
			updateSet{kbs: NewKBSet("5049296"), uuids: []string{otherUpdateID}},
			applyUnhide,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			if got := decideUnhide(tt.want, tt.hidden, cumulative()); got != tt.out {
				t.Errorf("decideUnhide() = %v, want %v", got, tt.out)
			}
		})
	}
}
