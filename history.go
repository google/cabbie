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
	"sort"

	"flag"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatehistory"
	"github.com/google/deck"
	"github.com/google/subcommands"
)

// Available flags
type historyCmd struct {
}

func (historyCmd) Name() string     { return "history" }
func (historyCmd) Synopsis() string { return "Get a list of all the installed updates on the device." }
func (historyCmd) Usage() string {
	return fmt.Sprintf("%s history\n", filepath.Base(os.Args[0]))
}
func (c *historyCmd) SetFlags(f *flag.FlagSet) {}

func (c *historyCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	h, err := history()
	if err != nil {
		fmt.Printf("Failed to get update history: %s", err)
		deck.ErrorfA("Failed to get Update history: %s", err).With(eventID(cablib.EvtErrHistory)).Go()
		return subcommands.ExitFailure
	}
	defer h.Close()

	// Print entries sorted by date
	sort.Slice(h.Entries, func(i, j int) bool {
		return h.Entries[i].Date.Before(h.Entries[j].Date)
	})

	for _, e := range h.Entries {
		fmt.Printf("Installed update:\n%v\n\n", e)
	}
	return subcommands.ExitSuccess
}

func history() (*updatehistory.History, error) {
	// Start Windows update session
	s, err := session.New()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	// Create Update searcher interface
	searcher, err := search.NewSearcher(s, "", config.WSUSServers, config.EnableThirdParty)
	if err != nil {
		return nil, err
	}
	defer searcher.Close()

	deck.InfoA("Collecting installed updates...").With(eventID(cablib.EvtHistory)).Go()
	return updatehistory.Get(searcher)
}
