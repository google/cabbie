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
	"github.com/google/cabbie/updates"
	"github.com/google/deck"
	"github.com/google/subcommands"
)

// Available flags
type hideCmd struct {
	kbs    string
	unhide bool
}

func (hideCmd) Name() string     { return "hide" }
func (hideCmd) Synopsis() string { return "hide available updates" }
func (hideCmd) Usage() string {
	return fmt.Sprintf("%s hide [--unhide] [--kbs=\"<KBnumber>\"]", filepath.Base(os.Args[0]))

}
func (c *hideCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.kbs, "kbs", "", "comma separated list of KB numbers to be hidden.")
	f.BoolVar(&c.unhide, "unhide", false, "mark a hidden update as visible.")
}

func (c hideCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	kbs := NewKBSet(c.kbs)

	if kbs.Size() < 1 {
		fmt.Printf("%s\nUsage: %s\n", c.Synopsis(), c.Usage())
		return subcommands.ExitUsageError
	}

	if c.unhide {
		if err := unhide(kbs); err != nil {
			fmt.Println(err)
			deck.ErrorfA("Error unhiding an update: %v", err).With(eventID(cablib.EvtErrUnhide)).Go()
		}
		return subcommands.ExitSuccess
	}

	if err := hide(kbs); err != nil {
		fmt.Println(err)
	}
	return subcommands.ExitSuccess
}

func modifyUpdatesVisibility(criteria string, evtID uint32, errEvtID uint32, actionMsg string, matchFn func(u *updates.Update) bool, actionFn func(u *updates.Update) error) error {
	uc, _, err := search.FindUpdates(nil, criteria, config.WSUSServers, config.EnableThirdParty)
	if err != nil {
		return err
	}
	defer uc.Close()

	deck.InfofA("Found %d matching updates.", len(uc.Updates)).With(eventID(evtID)).Go()

	for _, u := range uc.Updates {
		if matchFn(u) {
			deck.InfofA("%s update:\n%s", actionMsg, u.Title).With(eventID(evtID)).Go()
			if err := actionFn(u); err != nil {
				deck.ErrorfA("Failed to %s update %s:\n %s", strings.ToLower(actionMsg), u.Title, err).With(eventID(errEvtID)).Go()
			}
		}
	}

	return nil
}

func unhide(kbs KBSet) error {
	return modifyUpdatesVisibility(
		"IsHidden=1",
		cablib.EvtUnhide,
		cablib.EvtErrUnhide,
		"Unhiding",
		func(u *updates.Update) bool { return kbs.Search(u.KBArticleIDs) },
		func(u *updates.Update) error { return u.UnHide() },
	)
}

const nonHiddenCriteria = "IsHidden=0 and IsInstalled=0 or IsHidden=0 and IsInstalled=1"

func hide(kbs KBSet) error {
	return modifyUpdatesVisibility(
		nonHiddenCriteria,
		cablib.EvtHide,
		cablib.EvtErrHide,
		"Hiding",
		func(u *updates.Update) bool { return kbs.Search(u.KBArticleIDs) },
		func(u *updates.Update) error { return u.Hide() },
	)
}

func hideByUpdateID(uuids []string) error {
	uuidSet := make(map[string]bool, len(uuids))
	for _, id := range uuids {
		uuidSet[id] = true
	}
	return modifyUpdatesVisibility(
		nonHiddenCriteria,
		cablib.EvtHide,
		cablib.EvtErrHide,
		"Hiding",
		func(u *updates.Update) bool { return uuidSet[u.Identity.UpdateID] },
		func(u *updates.Update) error { return u.Hide() },
	)
}
