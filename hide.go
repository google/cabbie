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

	"flag"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
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

func unhide(kbs KBSet) error {
	// Find hidden updates.
	uc, err := findUpdates("IsHidden=1")
	if err != nil {
		return err
	}
	defer uc.Close()

	deck.InfofA("Found %d matching updates.", len(uc.Updates)).With(eventID(cablib.EvtUnhide)).Go()

	for _, u := range uc.Updates {
		if kbs.Search(u.KBArticleIDs) {
			deck.InfofA("Unhiding update:\n%s", u.Title).With(eventID(cablib.EvtUnhide)).Go()
			if err := u.UnHide(); err != nil {
				deck.ErrorfA("Failed to unhide update %s:\n %s", u.Title, err).With(eventID(cablib.EvtErrUnhide)).Go()
			}
		}
	}

	return nil
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
		for _, uuid := range uuids {
			if uuid == u.Identity.UpdateID {
				deck.InfofA("Hiding update by UpdateID:\n%s", u.Title).With(eventID(cablib.EvtHide)).Go()
				if err := u.Hide(); err != nil {
					deck.ErrorfA("Failed to hide update %s:\n %s", u.Title, err).With(eventID(cablib.EvtErrHide)).Go()
				}
			}
		}
	}

	return nil
}
