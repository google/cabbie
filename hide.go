// Copyright 2019 Google LLC
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
	"golang.org/x/net/context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"flag"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
	"github.com/google/cabbie/updates"
	"github.com/google/deck"
	"github.com/google/subcommands"
	"github.com/google/glazier/go/helpers"
)

const (
	// skipUnrelated indicates the update was not requested to be unhidden.
	skipUnrelated unhideAction = iota
	// skipConflict indicates the update was requested to be unhidden but is also
	// explicitly hidden, so it is left hidden.
	skipConflict
	// applyUnhide indicates the update should be made visible.
	applyUnhide
)

// unhideAction describes what unhide does with a single candidate update.
type unhideAction int

// Available flags
type hideCmd struct {
	kbs       string
	updateIDs string
	unhide    bool
}

// updateSet is a group of updates by KB, updateID, or both.
type updateSet struct {
	kbs   KBSet
	uuids []string
}

func (hideCmd) Name() string     { return "hide" }
func (hideCmd) Synopsis() string { return "hide available updates" }
func (hideCmd) Usage() string {
	return fmt.Sprintf("%s hide [--unhide] [--kbs=\"<KBnumber>\"] [--update-ids=\"<UpdateID>\"]", filepath.Base(os.Args[0]))

}
func (c *hideCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.kbs, "kbs", "", "comma separated list of KB numbers to be hidden.")
	f.StringVar(&c.updateIDs, "update-ids", "", "comma separated list of update IDs to be hidden.")
	f.BoolVar(&c.unhide, "unhide", false, "mark a hidden update as visible.")
}

func (c hideCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	kbs := NewKBSet(c.kbs)
	updateIDs := helpers.StringToSlice(c.updateIDs)

	if kbs.Size() < 1 && len(updateIDs) < 1 {
		fmt.Printf("%s\nUsage: %s\n", c.Synopsis(), c.Usage())
		return subcommands.ExitUsageError
	}

	if c.unhide {
		if err := unhide(updateSet{kbs: kbs, uuids: updateIDs}, updateSet{}); err != nil {
			fmt.Println(err)
			deck.ErrorfA("Error unhiding an update: %v", err).With(eventID(cablib.EvtErrUnhide)).Go()
		}
		return subcommands.ExitSuccess
	}

	if kbs.Size() > 0 {
		if err := hide(kbs); err != nil {
			fmt.Println(err)
		}
	}
	if len(updateIDs) > 0 {
		if err := hideByUpdateID(updateIDs); err != nil {
			fmt.Println(err)
		}
	}
	return subcommands.ExitSuccess
}

// TODO(cjgenevi): Turn into shared function that can be used by multiple actions
func findUpdates(criteria string) (*updatecollection.Collection, error) {
	// Start Windows update session
	s, err := session.New()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	q, err := search.NewSearcher(s, criteria, config.WSUSServers, config.EnableThirdParty)
	if err != nil {
		return nil, err
	}
	defer q.Close()

	return q.QueryUpdates()
}

// empty reports whether the set identifies no updates at all.
func (s updateSet) empty() bool {
	return s.kbs.Size() < 1 && len(s.uuids) < 1
}

// matches reports whether u is identified by this set, by either identifier.
func (s updateSet) matches(u *updates.Update) bool {
	return s.kbs.Search(u.KBArticleIDs) || matchUpdateID(s.uuids, u.Identity.UpdateID)
}

func (a unhideAction) String() string {
	switch a {
	case skipUnrelated:
		return "skipUnrelated"
	case skipConflict:
		return "skipConflict"
	case applyUnhide:
		return "applyUnhide"
	}
	return fmt.Sprintf("unhideAction(%d)", int(a))
}

// decideUnhide reports what should happen to u given the set of updates
// requested to be unhidden and the set of updates that are explicitly hidden.
// Hiding wins if both are specified.
func decideUnhide(want, hidden updateSet, u *updates.Update) unhideAction {
	switch {
	case !want.matches(u):
		return skipUnrelated
	case hidden.matches(u):
		return skipConflict
	default:
		return applyUnhide
	}
}

// unhide makes hidden updates visible again. An update is unhidden if it
// matches any of the passed KB article IDs or any of the passed update IDs.
func unhide(want, hidden updateSet) error {
	if want.empty() {
		return nil
	}

	// Find hidden updates.
	uc, err := findUpdates("IsHidden=1")
	if err != nil {
		return err
	}
	defer uc.Close()

	deck.InfofA("Found %d matching updates.", len(uc.Updates)).With(eventID(cablib.EvtUnhide)).Go()

	for _, u := range uc.Updates {
		switch decideUnhide(want, hidden, u) {
		case skipUnrelated:
			continue
		case skipConflict:
			deck.ErrorfA("Ignoring unhide for update %q (UpdateID: %s, KBs: %v) because it is also explicitly hidden.",
				u.Title, u.Identity.UpdateID, u.KBArticleIDs).With(eventID(cablib.EvtErrEnforcement)).Go()
			continue
		}
		deck.InfofA("Unhiding update:\n%s", u.Title).With(eventID(cablib.EvtUnhide)).Go()
		if err := u.UnHide(); err != nil {
			deck.ErrorfA("Failed to unhide update %s:\n %s", u.Title, err).With(eventID(cablib.EvtErrUnhide)).Go()
		}
	}

	return nil
}

// matchUpdateID reports whether updateID is present in uuids.
func matchUpdateID(uuids []string, updateID string) bool {
	for _, uuid := range uuids {
		if strings.EqualFold(uuid, updateID) {
			return true
		}
	}
	return false
}

func hide(kbs KBSet) error {
	// Find non-hidden updates that are installed or not installed.
	uc, err := findUpdates("IsHidden=0 and IsInstalled=0 or IsHidden=0 and IsInstalled=1")
	if err != nil {
		return err
	}
	defer uc.Close()

	deck.InfofA("Found %d matching updates.", len(uc.Updates)).With(eventID(cablib.EvtHide)).Go()

	for _, u := range uc.Updates {
		if kbs.Search(u.KBArticleIDs) {
			deck.InfofA("Hiding update:\n%s", u.Title).With(eventID(cablib.EvtHide)).Go()
			if err := u.Hide(); err != nil {
				deck.ErrorfA("Failed to hide update %s:\n %s", u.Title, err).With(eventID(cablib.EvtErrHide)).Go()
			}
		}
	}

	return nil
}

func hideByUpdateID(uuids []string) error {
	// Find non-hidden updates that are installed or not installed.
	uc, err := findUpdates("IsHidden=0 and IsInstalled=0 or IsHidden=0 and IsInstalled=1")
	if err != nil {
		return err
	}
	defer uc.Close()

	deck.InfofA("Found %d matching updates.", len(uc.Updates)).With(eventID(cablib.EvtHide)).Go()

	for _, u := range uc.Updates {
		if !matchUpdateID(uuids, u.Identity.UpdateID) {
			continue
		}
		deck.InfofA("Hiding update by UpdateID:\n%s", u.Title).With(eventID(cablib.EvtHide)).Go()
		if err := u.Hide(); err != nil {
			deck.ErrorfA("Failed to hide update %s:\n %s", u.Title, err).With(eventID(cablib.EvtErrHide)).Go()
		}
	}

	return nil
}
