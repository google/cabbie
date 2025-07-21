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
	"time"

	"flag"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/wsus"
	"github.com/google/deck"
	"github.com/google/subcommands"
	"github.com/google/glazier/go/helpers"
)

// wsusCmd defines the wsus subcommand.
type wsusCmd struct{}

func (wsusCmd) Name() string     { return "wsus" }
func (wsusCmd) Synopsis() string { return "Initialize and configure WSUS." }
func (wsusCmd) Usage() string {
	return fmt.Sprintf("%s wsus\n", filepath.Base(os.Args[0]))
}
func (c *wsusCmd) SetFlags(f *flag.FlagSet) {}

func (c *wsusCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {

	if _, err := wsus.Init(config.WSUSServers); err != nil {
		msg := fmt.Sprintf("Failed to initialize WSUS: %v\n", err)
		deck.ErrorfA("%s", msg).With(eventID(cablib.EvtErrMisc)).Go()
		fmt.Print(msg)
		return subcommands.ExitFailure
	}

	deck.InfoA("WSUS configuration refreshed, restarting wuauserv to apply settings.").With(eventID(cablib.EvtMisc)).Go()
	if err := helpers.RestartService("wuauserv"); err != nil {
		msg := fmt.Sprintf("Failed to restart wuauserv: %v\n", err)
		deck.ErrorfA("%s", msg).With(eventID(cablib.EvtErrMisc)).Go()
		fmt.Print(msg)
		return subcommands.ExitFailure
	}
	// Give it a few seconds to start.
	time.Sleep(time.Second * 5)

	fmt.Println("WSUS configuration refreshed and wuauserv restarted successfully.")
	return subcommands.ExitSuccess
}
