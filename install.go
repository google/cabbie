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
	"errors"
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
	"github.com/google/glazier/go/helpers"
)

// Available flags
type installCmd struct {
	all, drivers, deadlineOnly, Interactive, virusDef bool
	kbs                                               string
}

type installRsp struct {
	hResult        string
	resultCode     int
	rebootRequired bool
}

func (installCmd) Name() string     { return "install" }
func (installCmd) Synopsis() string { return "Install selected available updates." }
func (installCmd) Usage() string {
	return fmt.Sprintf("%s install [--drivers | --virusDef | --kbs=\"<KBNumber>\" | --all]\n", filepath.Base(os.Args[0]))
}

func (i *installCmd) SetFlags(f *flag.FlagSet) {
	// Category Flags
	f.BoolVar(&i.all, "all", false, "Install everything.")
	f.BoolVar(&i.drivers, "drivers", false, "Install available drivers.")
	f.BoolVar(&i.virusDef, "virus_def", false, "Update virus definitions.")
	f.StringVar(&i.kbs, "kbs", "", "Comma separated string of KB numbers in the form of 1234567.")

	// Behavior Flags
	f.BoolVar(&i.deadlineOnly, "deadlineOnly", false, fmt.Sprintf("Install available updates older than %d days", config.Deadline))
}

var errInvalidFlags = errors.New("invalid flag combination")

func vetFlags(i installCmd) error {
	f := 0
	for _, v := range []bool{i.all, i.drivers, i.virusDef, len(i.kbs) > 0} {
		if v {
			f++
		}
	}
	if f > 1 {
		fmt.Println("Multiple install flags can not be passed at the same time.")
		fmt.Printf("%s\nUsage: %s\n", i.Synopsis(), i.Usage())
		return errInvalidFlags
	}
	return nil
}

func (i installCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := vetFlags(i); err != nil {
		return subcommands.ExitUsageError
	}

	if err := i.installUpdates(); err != nil {
		fmt.Printf("Failed to install updates: %v", err)
		elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Failed to install updates: %v", err))
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
	case i.all:
		c = search.BasicSearch
		elog.Info(cablib.EvtSearch, fmt.Sprintf("Starting search for all updates: %s", c))
	case i.drivers:
		c = "Type='Driver'"
		rc = append(rc, "Drivers")
		elog.Info(cablib.EvtSearch, fmt.Sprintf("Starting search for updated drivers: %s", c))
	case i.virusDef:
		c = fmt.Sprintf("%s AND CategoryIDs contains '%s'", search.BasicSearch, search.DefinitionUpdates)
		rc = append(rc, "Definition Updates")
		elog.Info(cablib.EvtSearch, fmt.Sprintf("Starting search for virus definitions:\n%s", c))
	case i.kbs != "":
		c = search.BasicSearch
		elog.Info(cablib.EvtSearch, fmt.Sprintf("Starting search for KB's %q:\n%s", i.kbs, c))
	default:
		c = search.BasicSearch
		rc = config.RequiredCategories
		elog.Info(cablib.EvtSearch, fmt.Sprintf("Starting search for general updates: %s", c))
	}
	return c, rc
}

func installingMessage() {
	elog.Info(cablib.EvtInstall, "Cabbie is installing new updates.")

	if err := notification.NewInstallingMessage().Push(); err != nil {
		elog.Error(cablib.EvtErrNotifications, fmt.Sprintf("Failed to create notification:\n%v", err))
	}
}

func rebootMessage(seconds int) {
	elog.Info(cablib.EvtInstallSuccess, "Updates have been installed, please reboot to complete the installation...")

	if err := notification.NewRebootMessage(seconds).Push(); err != nil {
		elog.Error(cablib.EvtErrNotifications, fmt.Sprintf("Failed to create notification:\n%v", err))
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
		elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting metric:\n%v", er))
	}
	if err != nil {
		return fmt.Errorf("error encountered when attempting to query for updates: %v", err)
	}
	defer uc.Close()

	if len(uc.Updates) == 0 {
		elog.Info(cablib.EvtNoUpdates, "No updates found to install.")
		return nil
	}
	elog.Info(cablib.EvtUpdatesFound, fmt.Sprintf("Updates Found:\n%s", strings.Join(uc.Titles(), "\n\n")))

	installMsgPopped := i.virusDef
	installingMinOneUpdate := false

	kbs := NewKBSet(i.kbs)
	for _, u := range uc.Updates {
		if !(u.InCategories(rc)) {
			elog.Info(cablib.EvtUpdateSkip, fmt.Sprintf("Skipping update %s.\nRequiredClassifications:\n%v\nUpdate classifications:\n%v",
				u.Title,
				rc,
				u.Categories))
			continue
		}

		if !(u.EulaAccepted) {
			elog.Info(cablib.EvtMisc, fmt.Sprintf("Accepting EULA for update: %s", u.Title))
			if err := u.AcceptEula(); err != nil {
				elog.Error(cablib.EvtErrMisc, fmt.Sprintf("Failed to accept EULA for update %s:\n%s", u.Title, err))
			}
		}

		if kbs.Size() > 0 {
			if !kbs.Search(u.KBArticleIDs) {
				elog.Info(cablib.EvtUpdateSkip, fmt.Sprintf("Skipping update %s.\nRequired KBs:\n%s\nUpdate KBs:\n%v",
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
				elog.Info(cablib.EvtUpdateSkip,
					fmt.Sprintf("Skipping update %s.\nUpdate deployed on %v has not reached the %d day threshold.",
						u.Title,
						u.LastDeploymentChangeTime,
						config.Deadline))
				continue
			}
		}

		c, err := updatecollection.New()
		if err != nil {
			elog.Error(cablib.EvtErrMisc, fmt.Sprintf("Failed to create collection: %v", err))
			continue
		}
		c.Add(u.Item)

		if !installMsgPopped && !u.InCategories([]string{"Definition Updates"}) {
			installingMessage()
			installMsgPopped = true
			ps := filepath.Join(cablib.CabbiePath, "PreUpdate.ps1")
			exist, err := helpers.PathExists(ps)
			if err != nil {
				elog.Error(cablib.EvtErrUpdateScript, fmt.Sprintf("PreUpdateScript: error checking existence of %q:\n%v", cablib.CabbiePath+"PreUpdate.ps1", err))
			} else if exist {
				if _, err := helpers.ExecWithVerify(ps, nil, &config.ScriptTimeout, nil); err != nil {
					elog.Error(cablib.EvtErrUpdateScript, fmt.Sprintf("PreUpdateScript: error running script:\n%v", err))
				}
			}
			installingMinOneUpdate = true
		}
		elog.Info(cablib.EvtDownload, fmt.Sprintf("Downloading Update:\n%v", u))

		rc, err := downloadCollection(s, c)
		if err != nil {
			elog.Error(cablib.EvtErrMisc, err.Error())
			c.Close()
			continue
		}
		if rc == 2 {
			elog.Info(cablib.EvtDownload, fmt.Sprintf("Successfully downloaded update:\n %s", u.Title))
		} else {

			elog.Error(cablib.EvtErrDownloadFailure, fmt.Sprintf("Failed to download update:\n %s\n ReturnCode: %d", u.Title, rc))
			c.Close()
			continue
		}

		elog.Info(cablib.EvtInstall, fmt.Sprintf("Installing Update:\n%v", u))

		rsp, err := installCollection(s, c)
		if err != nil {
			elog.Error(cablib.EvtErrMisc, err.Error())
			c.Close()
			continue
		}

		if err := installHResult.Set(rsp.hResult); err != nil {
			elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting metric:\n%v", err))
		}
		if rsp.resultCode == 2 {
			elog.Info(cablib.EvtInstall, fmt.Sprintf("Successfully installed update:\n%s\nHResult Code: %s", u.Title, rsp.hResult))
		} else {
			elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Failed to install update:\n%s\nReturnCode: %d\nHResult Code: %s", u.Title, rsp.resultCode, rsp.hResult))
			c.Close()
			continue
		}

		elog.Info(cablib.EvtRebootRequired, fmt.Sprintf("Install Reboot Required: %t", rsp.rebootRequired))
		if !rebootRequired {
			rebootRequired = rsp.rebootRequired
		}
		c.Close()
	}

	if installingMinOneUpdate {
		ps := filepath.Join(cablib.CabbiePath, "PostUpdate.ps1")
		exist, err := helpers.PathExists(ps)
		if err != nil {
			elog.Error(cablib.EvtErrUpdateScript, fmt.Sprintf("PostUpdateScript: error checking existence of %q:\n%v", cablib.CabbiePath+"PostUpdate.ps1", err))
		} else if exist {
			if _, err := helpers.ExecWithVerify(ps, nil, &config.ScriptTimeout, nil); err != nil {
				elog.Error(cablib.EvtErrUpdateScript, fmt.Sprintf("PostUpdateScript: error executing script:\n%v", err))
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
			elog.Error(cablib.EvtErrPowerMgmt, fmt.Sprintf("Failed to run reboot command:\n%v", err))
		}
		rebootEvent <- rebootRequired
	}

	return nil
}
