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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"flag"
	"github.com/google/cabbie/notification"
	"github.com/google/cabbie/cablib"
	"github.com/google/deck/backends/eventlog"
	"github.com/google/deck"
	"golang.org/x/sys/windows/registry"
	"github.com/google/subcommands"
)

// Available flags
type rebootCmd struct {
	clear bool
	time  uint64 // time in seconds until reboot
	check bool
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
	f.BoolVar(&c.check, "check", false, "get if a reboot is pending.")
}

func (c rebootCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	eventID := eventlog.EventID
	rc := subcommands.ExitSuccess
	if !c.clear && c.time == 0 && !c.check {
		fmt.Println(c.Usage())
		fmt.Println("Either --clear, --time (non-zero), or --check must be set.")
		return subcommands.ExitFailure
	}
	if c.clear {
		if err := notification.CleanNotifications(cablib.SvcName); err != nil {
			deck.ErrorfA("Failed to clear reboot notification: %v", err).With(eventID(cablib.EvtErrNotifications)).Go()
		}
		if err := cablib.ClearRebootTime(); err != nil {
			if errors.Is(err, registry.ErrNotExist) {
				fmt.Printf("No Cabbie reboot time found to clear.")
				return subcommands.ExitSuccess
			}
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
	if c.check {
		pending, err := cablib.RebootRequired()
		if err != nil {
			msg := fmt.Sprintf("Failed to get reboot pending status: %v", err)
			deck.ErrorfA(msg).With(eventID(cablib.EvtMisc)).Go()
			fmt.Printf(msg)
			return subcommands.ExitFailure
		}
		if !pending {
			msg := "No reboot is pending."
			deck.InfoA(msg).With(eventID(cablib.EvtMisc)).Go()
			fmt.Println(msg)
			return rc
		}
		rebootTime, err := cablib.RebootTime()
		if err != nil {
			msg := fmt.Sprintf("A reboot is pending, but failed to get reboot time: %v", err)
			deck.ErrorfA(msg).With(eventID(cablib.EvtMisc)).Go()
			fmt.Printf(msg)
			return subcommands.ExitFailure
		}
		msg := fmt.Sprintf("A reboot is pending at %s.\n", rebootTime.String())
		deck.InfoA(msg).With(eventID(cablib.EvtMisc)).Go()
		fmt.Print(msg)
	}
	return rc
}
