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
	"github.com/google/cabbie/notification"
	"github.com/google/cabbie/cablib"
	"github.com/google/deck/backends/eventlog"
	"github.com/google/deck"
	"github.com/google/subcommands"
)

// Available flags
type rebootCmd struct {
	clear bool
	time  uint64 // time in seconds until reboot
}

func (rebootCmd) Name() string { return "reboot" }
func (rebootCmd) Synopsis() string {
	return "manually set or clear the Cabbie reboot time Registry key."
}
func (rebootCmd) Usage() string {
	return fmt.Sprintf("%s reboot [--clear] [--time <seconds>]\n", filepath.Base(os.Args[0]))
}
func (c *rebootCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.clear, "clear", false, "clear the reboot time if set.")
	f.Uint64Var(&c.time, "time", 0, "set the reboot time in seconds.")
}

func (c rebootCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	eventID := eventlog.EventID
	rc := subcommands.ExitSuccess
	if !c.clear && c.time == 0 {
		fmt.Println(c.Usage())
		fmt.Println("Either --clear or --time (non-zero) must be set.")
		return subcommands.ExitFailure
	}
	if c.clear {
		if err := notification.CleanNotifications(cablib.SvcName); err != nil {
			deck.ErrorfA("Failed to clear reboot notification: %v", err).With(eventID(cablib.EvtErrNotifications)).Go()
		}
		if err := cablib.ClearRebootTime(); err != nil {
			fmt.Printf("Failed to clear reboot time: %v", err)
			return subcommands.ExitFailure
		}
		msg := "Cabbie reboot time has been manually cleared."
		deck.InfoA(msg).With(eventID(cablib.EvtRebootRequired)).Go()
		fmt.Print(msg)
		return rc
	}
	if c.time != 0 {
		rebootTime := time.Now().Add(time.Second * time.Duration(c.time))
		if err := notification.NewRebootMessage(rebootTime).Push(); err != nil {
			deck.ErrorfA("Failed to create manually set reboot notification: %v", err).With(eventID(cablib.EvtErrNotifications)).Go()
		}
		if err := cablib.SetRebootTime(rebootTime); err != nil {
			deck.ErrorfA("Failed to set reboot time: %v", err).With(eventID(cablib.EvtRebootRequired)).Go()
			fmt.Printf("Failed to set reboot time: %v", err)
			return subcommands.ExitFailure
		}
		msg := fmt.Sprintf("Cabbie reboot time has been manually set to %v", rebootTime)
		deck.InfoA(msg).With(eventID(cablib.EvtRebootRequired)).Go()
		fmt.Print(msg)
	}
	return rc
}
