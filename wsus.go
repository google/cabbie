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
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flag"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/wsus"
	"github.com/google/deck"
	"golang.org/x/sys/windows/registry"
	"github.com/google/subcommands"
	"github.com/google/glazier/go/helpers"
)

var (
	// Test Stubs
	netDialTimeout = net.DialTimeout
)

// wsusCmd defines the wsus subcommand.
type wsusCmd struct {
	wsusServers string
	force       bool
}

func (wsusCmd) Name() string     { return "wsus" }
func (wsusCmd) Synopsis() string { return "Initialize and configure WSUS." }
func (wsusCmd) Usage() string {
	return fmt.Sprintf("%s wsus [--wsus_servers=<server1>,<server2>]\n", filepath.Base(os.Args[0]))
}
func (c *wsusCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.wsusServers, "wsus_servers", "", "Comma-separated list of WSUS servers to configure if Windows Update is unavailable.")
	f.BoolVar(&c.force, "force", false, "Force the WSUS configuration even if Windows Update is available.")
}

const wuHost = "windowsupdate.microsoft.com"

func setWsusIfNeeded(targets string, force bool) error {
	// If we're already connected to WU, we don't need to do anything.
	conn, err := netDialTimeout("tcp", net.JoinHostPort(wuHost, "80"), 3*time.Second)
	if err == nil && !force {
		conn.Close()
		return nil
	}
	deck.Infof("Failed to connect to WU or force flag was set: `%s`.. Configuring WSUS", err)
	var servers []string
	for _, s := range strings.Split(targets, ",") {
		if s != "" {
			servers = append(servers, s)
		}
	}
	if len(servers) == 0 {
		return fmt.Errorf("no valid WSUS servers provided in targets string %q", targets)
	}
	deck.Info("Writing WSUS servers to registry.")
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, cablib.RegPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("registry.CreateKey(%s): %w", cablib.RegPath, err)
	}
	defer k.Close()
	if err := k.SetStringsValue("WSUSServers", servers); err != nil {
		return fmt.Errorf("registry.SetStringsValue(WSUSServers): %w", err)
	}

	// If we wrote to registry, we should reload config so that wsus.Init gets new servers.
	deck.Info("Reloading config to apply WSUS servers.")
	if err := config.regLoad(cablib.RegPath); err != nil {
		deck.ErrorfA("Failed to reload Cabbie config after setting WSUS servers:\n%v\nError:%v", config, err).With(eventID(cablib.EvtErrConfig)).Go()
	}
	return nil
}

func (c *wsusCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	if c.wsusServers != "" {
		if err := setWsusIfNeeded(c.wsusServers, c.force); err != nil {
			msg := fmt.Sprintf("Failed to set WSUS servers: %v\n", err)
			deck.ErrorfA("%s", msg).With(eventID(cablib.EvtErrMisc)).Go()
			fmt.Print(msg)
			return subcommands.ExitFailure
		}
	}

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
