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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flag"
	"github.com/google/cabbie/notification"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/download"
	"github.com/google/cabbie/install"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
	"github.com/google/subcommands"
)

// Available flags
type installCmd struct {
	drivers, deadlineOnly, Interactive, virusDef bool
	kbs                                          string
}

type installRsp struct {
	hResult        string
	resultCode     int
	rebootRequired bool
}

func (installCmd) Name() string     { return "install" }
func (installCmd) Synopsis() string { return "Install selected available updates." }
func (installCmd) Usage() string {
	return fmt.Sprintf("%s install [--drivers | --virusDef | --kbs=\"<KBNumber>\"]\n", filepath.Base(os.Args[0]))
}

func (i *installCmd) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&i.drivers, "drivers", false, "Install available drivers.")
	f.BoolVar(&i.virusDef, "virus_def", false, "Update virus definitions.")
	f.BoolVar(&i.deadlineOnly, "deadlineOnly", false, fmt.Sprintf("Install available updates older than %d days", config.Deadline))
	f.StringVar(&i.kbs, "kbs", "", "Comma separated string of KB numbers in the form of 1234567.")
}

func (i installCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	// TODO: Fix logic to allow only 0 to 1 flags at a time.
	if i.drivers && i.virusDef && i.kbs != "" {
		fmt.Println("drivers and virus_def flags can not be passed at the same time.")
		fmt.Printf("%s\nUsage: %s\n", i.Synopsis(), i.Usage())
		return subcommands.ExitUsageError
	}

	if err := i.installUpdates(); err != nil {
		fmt.Printf("Failed to install updates: %v", err)
		elog.Error(113, fmt.Sprintf("Failed to install updates: %v", err))
		return subcommands.ExitFailure
	}

	select {
	case <-rebootEvent:
		fmt.Println("Please reboot to finalize the update installation.")
		return 6
	default:
		fmt.Println("No reboot needed.")
	}

	return subcommands.ExitSuccess
}

func (i *installCmd) criteria() (string, []string) {
	// Set search criteria and required categories.
	var c string
	var rc []string
	switch {
	case i.drivers:
		c = "Type='Driver'"
		rc = append(rc, "Drivers")
		elog.Info(0021, fmt.Sprintf("Starting search for updated drivers: %s", c))
	case i.virusDef:
		c = fmt.Sprintf("%s AND CategoryIDs contains '%s'", search.BasicSearch, search.DefinitionUpdates)
		rc = append(rc, "Definition Updates")
		elog.Info(0022, fmt.Sprintf("Starting search for virus definitions:\n%s", c))
	case i.kbs != "":
		c = search.BasicSearch
		elog.Info(0023, fmt.Sprintf("Starting search for KB's %q:\n%s", i.kbs, c))
	default:
		c = search.BasicSearch
		rc = config.RequiredCategories
		elog.Info(0024, fmt.Sprintf("Starting search for general updates: %s", c))
	}
	return c, rc
}

func installingMessage() {
	elog.Info(2, "Cabbie is installing new updates.")

	if err := notification.NewNotification(cablib.SvcName, notification.NewInstallingMessage(), "installingUpdates"); err != nil {
		elog.Error(6, fmt.Sprintf("Failed to create notification:\n%v", err))
	}
}

func rebootMessage(seconds int) {
	elog.Info(2, "Updates have been installed, please reboot to complete the installation...")

	if err := notification.NewNotification(cablib.SvcName, notification.NewRebootMessage(seconds), "rebootPending"); err != nil {
		elog.Error(6, fmt.Sprintf("Failed to create notification:\n%v", err))
	}
}

func downloadCollection(s *session.UpdateSession, c *updatecollection.Collection) (int, error) {
	d, err := download.NewDownloader(s, c)
	if err != nil {
		return 0, fmt.Errorf("error creating downloader:\n %v", err)
	}
	defer d.Close()

	if err := d.Download(); err != nil {
		return 0, fmt.Errorf("error downloading updates:\n %v", err)
	}

	return d.ResultCode()
}

func installCollection(s *session.UpdateSession, c *updatecollection.Collection) (*installRsp, error) {
	inst, err := install.NewInstaller(s, c)
	if err != nil {
		return nil, fmt.Errorf("error creating installer: \n %v", err)
	}
	defer inst.Close()

	if err := inst.Install(); err != nil {
		return nil, fmt.Errorf("error installing updates:\n %v", err)
	}

	rc, err := inst.ResultCode()
	if err != nil {
		return nil, fmt.Errorf("error getting install ResultCode:\n %v", err)
	}

	hr, err := inst.HResult()
	if err != nil {
		return nil, fmt.Errorf("error getting install ReturnCode:\n %v", err)
	}

	rb, err := inst.RebootRequired()

	return &installRsp{
		hResult:        hr,
		resultCode:     rc,
		rebootRequired: rb,
	}, err
}

func (i *installCmd) installUpdates() error {
	var rebootRequired bool

	// Check for reboot status when not installing virus definitions.
	if !(i.virusDef) {
		rebootRequired, err := cablib.RebootRequired()
		if err != nil {
			return fmt.Errorf("failed to determine reboot status: %v", err)
		}

		if rebootRequired {
			if i.Interactive {
				fmt.Println("Host has existing updates pending reboot.")
				return nil
			}
			t, err := cablib.RebootTime()
			if err != nil {
				return fmt.Errorf("Error getting reboot time: %v", err)
			}
			if t.IsZero() {
				// Set reboot time if a reboot is pending but no time has been set.
				// This can happen when a user installs updates outside of Cabbie.
				rebootMessage(int(config.RebootDelay))
				if err := cablib.SetRebootTime(config.RebootDelay); err != nil {
					return fmt.Errorf("Failed to set reboot time:\n%v", err)
				}
			}
			rebootEvent <- rebootRequired
			return nil
		}
	}

	// Start Windows update session
	s, err := session.New()
	if err != nil {
		return fmt.Errorf("failed to create new Windows Update session: %v", err)
	}
	defer s.Close()

	criteria, rc := i.criteria()

	q, err := search.NewSearcher(s, criteria, config.WSUSServers, config.EnableThirdParty)
	if err != nil {
		return fmt.Errorf("failed to create a new searcher object: %v", err)
	}
	defer q.Close()

	uc, err := q.QueryUpdates()
	if er := searchHResult.Set(q.SearchHResult); er != nil {
		elog.Error(206, fmt.Sprintf("Error posting metric:\n%v", er))
	}
	if err != nil {
		return fmt.Errorf("error encountered when attempting to query for updates: %v", err)
	}
	defer uc.Close()

	if len(uc.Updates) == 0 {
		elog.Info(002, "No updates found to install.")
		return nil
	}
	elog.Info(4, fmt.Sprintf("Updates Found:\n%s", strings.Join(uc.Titles(), "\n\n")))

	installMsgPopped := i.virusDef
	installingMinOneUpdate := false

	kbs := NewKBSet(i.kbs)
	for _, u := range uc.Updates {
		if !(u.InCategories(rc)) {
			elog.Info(1, fmt.Sprintf("Skipping update %s.\nRequiredClassifications:\n%v\nUpdate classifications:\n%v",
				u.Title,
				rc,
				u.Categories))
			continue
		}

		if !(u.EulaAccepted) {
			elog.Info(002, fmt.Sprintf("Accepting EULA for update: %s", u.Title))
			if err := u.AcceptEula(); err != nil {
				elog.Error(202, fmt.Sprintf("Failed to accept EULA for update %s:\n%s", u.Title, err))
			}
		}

		if kbs.Size() > 0 {
			if !kbs.Search(u.KBArticleIDs) {
				elog.Info(1, fmt.Sprintf("Skipping update %s.\nRequired KBs:\n%s\nUpdate KBs:\n%v",
					u.Title,
					kbs,
					u.KBArticleIDs))
				continue
			}
		}
		if i.deadlineOnly {
			deadline := time.Duration(config.Deadline) * 24 * time.Hour
			pastDeadline := time.Now().After(u.LastDeploymentChangeTime.Add(deadline))
			if !pastDeadline {
				elog.Info(002,
					fmt.Sprintf("Skipping update %s.\nUpdate deployed on %v has not reached the %d day threshold.",
						u.Title,
						u.LastDeploymentChangeTime,
						config.Deadline))
				continue
			}
		}

		c, err := updatecollection.New()
		if err != nil {
			elog.Error(202, fmt.Sprintf("Failed to create collection: %v", err))
			continue
		}
		c.Add(u.Item)

		if !installMsgPopped && !u.InCategories([]string{"Definition Updates"}) {
			installingMessage()
			installMsgPopped = true

			exist, err := cablib.PathExists(filepath.Join(cablib.CabbiePath, "PreUpdate.ps1"))
			if err != nil {
				elog.Error(207, fmt.Sprintf("PreUpdateScript: error checking existence of %q:\n%v", cablib.CabbiePath+"PreUpdate.ps1", err))
			} else if exist {
				if err := cablib.RunScript("PreUpdate.ps1", config.ScriptTimeout); err != nil {
					elog.Error(208, fmt.Sprintf("PreUpdateScript: error running script:\n%v", err))
				}
			}
			installingMinOneUpdate = true
		}
		elog.Info(002, fmt.Sprintf("Downloading Update:\n%v", u))

		rc, err := downloadCollection(s, c)
		if err != nil {
			elog.Error(203, fmt.Sprintf("%v", err))
			c.Close()
			continue
		}
		if rc == 2 {
			elog.Info(002, fmt.Sprintf("Successfully downloaded update:\n %s", u.Title))
		} else {

			elog.Error(204, fmt.Sprintf("Failed to download update:\n %s\n ReturnCode: %d", u.Title, rc))
			c.Close()
			continue
		}

		elog.Info(002, fmt.Sprintf("Installing Update:\n%v", u))

		rsp, err := installCollection(s, c)
		if err != nil {
			elog.Error(205, fmt.Sprintf("%v", err))
			c.Close()
			continue
		}

		if err := installHResult.Set(rsp.hResult); err != nil {
			elog.Error(206, fmt.Sprintf("Error posting metric:\n%v", err))
		}
		if rsp.resultCode == 2 {
			elog.Info(002, fmt.Sprintf("Successfully installed update:\n%s\nHResult Code: %s", u.Title, rsp.hResult))
		} else {
			elog.Error(206, fmt.Sprintf("Failed to install update:\n%s\nReturnCode: %d\nHResult Code: %s", u.Title, rsp.resultCode, rsp.hResult))
			c.Close()
			continue
		}

		elog.Info(002, fmt.Sprintf("Install Reboot Required: %t", rsp.rebootRequired))
		if !rebootRequired {
			rebootRequired = rsp.rebootRequired
		}
		c.Close()
	}

	if installingMinOneUpdate {
		exist, err := cablib.PathExists(filepath.Join(cablib.CabbiePath, "PostUpdate.ps1"))
		if err != nil {
			elog.Error(307, fmt.Sprintf("PostUpdateScript: error checking existence of %q:\n%v", cablib.CabbiePath+"PostUpdate.ps1", err))
		} else if exist {
			if err := cablib.RunScript("PostUpdate.ps1", config.ScriptTimeout); err != nil {
				elog.Error(308, fmt.Sprintf("PostUpdateScript: error executing script:\n%v", err))
			}
		}
	}

	if rebootRequired {
		if i.Interactive {
			fmt.Println("Updates have been installed, please reboot to complete the installation...")
			return nil
		}
		rebootMessage(int(config.RebootDelay))
		if err := cablib.SetRebootTime(config.RebootDelay); err != nil {
			elog.Error(306, fmt.Sprintf("Failed to run reboot command:\n%v", err))
		}
		rebootEvent <- rebootRequired
	}

	return nil
}
