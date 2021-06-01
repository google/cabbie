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
	"time"

	"flag"
	"github.com/google/cabbie/cablib"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.org/x/sys/windows/svc"
	"github.com/google/subcommands"
)

// Available flags.
type serviceCmd struct {
	install   bool
	uninstall bool
}

func (serviceCmd) Name() string     { return "service" }
func (serviceCmd) Synopsis() string { return "Manage the installation status of the Cabbie service." }
func (serviceCmd) Usage() string {
	return fmt.Sprintf("%s service [--install | --uninstall]\n", filepath.Base(os.Args[0]))
}
func (c *serviceCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.install, "install", false, "Install the Cabbie service.")
	f.BoolVar(&c.uninstall, "uninstall", false, "Uninstall the Cabbie service.")
}

func (c serviceCmd) Execute(ctx context.Context, flags *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	rc := subcommands.ExitSuccess

	if c.install && c.uninstall {
		fmt.Println("Install and Uninstall flags can not be passed at the same time.")
		return subcommands.ExitFailure
	}

	if c.install {
		if err := installService(cablib.SvcName, cablib.SvcName+" Update Manager"); err != nil {
			msg := fmt.Sprintf("Failed to install service: %v\n", err)
			elog.Error(cablib.EvtErrSvcInstall, msg)
			fmt.Println(msg)
			rc = subcommands.ExitFailure
		}
		elog.Info(cablib.EvtSvcInstall, "Successfully installed Cabbie service.")
	}
	if c.uninstall {
		if err := removeService(cablib.SvcName); err != nil {
			msg := fmt.Sprintf("Failed to uninstall service: %v\n", err)
			elog.Error(cablib.EvtErrSvcInstall, msg)
			fmt.Println(msg)
			rc = subcommands.ExitFailure
		}
		elog.Info(cablib.EvtSvcInstall, "Successfully uninstalled Cabbie service.")
	}

	if !(c.install || c.uninstall) {
		fmt.Printf("%s\nUsage: %s\n", c.Synopsis(), c.Usage())
		rc = subcommands.ExitUsageError
	}
	return rc
}

func installService(name, desc string) error {
	exepath, err := filepath.Abs(cablib.CabbiePath + cablib.CabbieExe)
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	// Generate cabbie service config.
	config := mgr.Config{
		DisplayName: desc,
		StartType:   mgr.StartAutomatic,
	}

	// Install or update Cabbie service.
	s, err := m.OpenService(name)
	if err == nil {
		msg := fmt.Sprintf("service %q already exists. Updating service config and ensuring service is running...\n", name)
		elog.Info(cablib.EvtSvcInstall, msg)
		fmt.Println(msg)
		s.UpdateConfig(config)
	} else {
		s, err = m.CreateService(name, exepath, config)
		if err != nil {
			return err
		}

		if err = eventlog.InstallAsEventCreate(cablib.LogSrcName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
			msg := fmt.Sprintf("event log source creation failed: %+v", err)
			elog.Error(cablib.EvtErrSvcInstall, msg)
			fmt.Println(msg)
		}

	}
	defer s.Close()

	// Set service recovery actions.
	ra := []mgr.RecoveryAction{
		{
			Type:  mgr.ServiceRestart,
			Delay: 5 * time.Second,
		},
		{
			Type:  mgr.ServiceRestart,
			Delay: 5 * time.Second,
		},
		{
			Type:  mgr.ServiceRestart,
			Delay: 5 * time.Second,
		},
	}
	if err := s.SetRecoveryActions(ra, 60); err != nil {
		msg := fmt.Sprintf("Failed to set service recovery actions:\n%v", err)
		elog.Error(cablib.EvtErrSvcInstall, msg)
		fmt.Println(msg)
	}

	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("failed to query service: %v", err)
	}

	if status.State == svc.Running {
		return nil
	}

	fmt.Println("Starting service...")
	return s.Start()
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		msg := fmt.Sprintf("service %q is not installed.", name)
		elog.Info(cablib.EvtSvcInstall, msg)
		fmt.Println(msg)
		return nil
	}
	defer s.Close()
	if err = s.Delete(); err != nil {
		return err
	}

	_, err = s.Control(svc.Stop)
	if err != nil {
		msg := fmt.Sprintf("Failed to stop service:\n%v", err)
		elog.Error(cablib.EvtErrService, msg)
		fmt.Println(msg)
	}

	if err = eventlog.Remove(name); err != nil {
		return fmt.Errorf("event log removal failed: %s", err)
	}
	return nil
}
