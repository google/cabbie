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
	"golang.org/x/net/context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flag"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/session"
	"github.com/google/subcommands"
)

// Available flags
type listCmd struct {
	hidden bool
}

func (listCmd) Name() string     { return "list" }
func (listCmd) Synopsis() string { return "list updates available for install." }
func (listCmd) Usage() string {
	return fmt.Sprintf("%s list [--hidden]\n", filepath.Base(os.Args[0]))

}
func (c *listCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.hidden, "hidden", false, "show updates that have been marked as hidden.")
}

func (c listCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	rc := subcommands.ExitSuccess
	var requiredUpdates, optionalUpdates []string
	var err error
	requiredUpdates, optionalUpdates, err = listUpdates(c.hidden)
	if err != nil {
		fmt.Printf("failed to get updates with error:\n%v\n", err)
		rc = subcommands.ExitFailure
	}
	msg := fmt.Sprintf("Found %d required updates.\nRequired updates:\n%s\nOptional updates:\n%s\n",
		len(requiredUpdates), strings.Join(requiredUpdates, "\n"), strings.Join(optionalUpdates, "\n"))
	elog.Info(cablib.EvtList, msg)
	fmt.Print(msg)
	return rc
}

// listUpdates queries the update server and returns a list of available updates
func listUpdates(hidden bool) ([]string, []string, error) {
	// Set search criteria
	c := search.BasicSearch + " OR Type='Driver' OR " + search.BasicSearch + " AND Type='Software'"
	if hidden {
		c += " and IsHidden=1"
	} else {
		c += " and IsHidden=0"
	}

	// Start Windows update session
	s, err := session.New()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new Windows Update session: %v", err)
	}
	defer s.Close()

	q, err := search.NewSearcher(s, c, config.WSUSServers, config.EnableThirdParty)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a new searcher object: %v", err)
	}
	defer q.Close()

	elog.Info(cablib.EvtSearch, fmt.Sprintf("Using search criteria: %s\n", q.Criteria))
	uc, err := q.QueryUpdates()
	if err != nil {
		return nil, nil, fmt.Errorf("error encountered when attempting to query for updates: %v", err)
	}
	defer uc.Close()

	var reqUpdates, optUpdates []string
	devicePatched := true
	for _, u := range uc.Updates {
		// Add to optional updates list if the update does not match the required categories.
		if !u.InCategories(config.RequiredCategories) {
			optUpdates = append(optUpdates, u.Title)
			continue
		}
		// Skip virus updates as they always exist.
		if !u.InCategories([]string{"Definition Updates"}) {
			reqUpdates = append(reqUpdates, u.Title)
			if (time.Now().Sub(u.LastDeploymentChangeTime).Hours() / 24) > 31 {
				if devicePatched == true {
					devicePatched = false
				}
			}
		}
	}
	deviceIsPatched.Set(devicePatched)
	return reqUpdates, optUpdates, nil
}
