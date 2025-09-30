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
	"github.com/google/deck"
	"github.com/google/aukera/client"
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

var (
	errInvalidFlags = errors.New("invalid flag combination")
	rebootList      = []string{}
	rebootTime      time.Time
)

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

func (i installCmd) Execute(_ context.Context, flags *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	if err := vetFlags(i); err != nil {
		return subcommands.ExitUsageError
	}

	if err := i.installUpdates(); err != nil {
		fmt.Printf("Failed to install updates: %v", err)
		deck.ErrorfA("Failed to install updates: %v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
		return subcommands.ExitFailure
	}

	select {
	case <-rebootEvent:
		fmt.Println("Please reboot to finalize the update installation.")
		return 6
	default:
		fmt.Println("Installation complete; no reboot required.")
	}

	return subcommands.ExitSuccess
}

func (i *installCmd) criteria() (string, []string) {
	// Set search criteria and required categories.
	var c string
	var rc []string
	switch {
	case i.all:
		c = search.BasicSearch + " AND IsHidden=0 OR Type='Driver'"
		deck.InfofA("Starting search for all updates: %s", c).With(eventID(cablib.EvtSearch)).Go()
	case i.drivers:
		c = "Type='Driver'"
		rc = append(rc, "Drivers")
		deck.InfofA("Starting search for updated drivers: %s", c).With(eventID(cablib.EvtSearch)).Go()
	case i.virusDef:
		c = fmt.Sprintf("%s AND CategoryIDs contains '%s'", search.BasicSearch, search.DefinitionUpdates)
		rc = append(rc, "Definition Updates")
		deck.InfofA("Starting search for virus definitions:\n%s", c).With(eventID(cablib.EvtSearch)).Go()
	case i.kbs != "":
		c = search.BasicSearch
		deck.InfofA("Starting search for KB's %q:\n%s", i.kbs, c).With(eventID(cablib.EvtSearch)).Go()
	default:
		c = search.BasicSearch + " AND IsHidden=0 OR Type='Driver'"
		rc = config.RequiredCategories
		deck.InfofA("Starting search for general updates: %s", c).With(eventID(cablib.EvtSearch)).Go()
	}
	return c, rc
}

func installingMessage() {
	deck.InfoA("Cabbie is installing new updates.").With(eventID(cablib.EvtInstall)).Go()

	if err := notification.NewInstallingMessage().Push(); err != nil {
		deck.ErrorfA("Failed to create notification:\n%v", err).With(eventID(cablib.EvtErrNotifications)).Go()
	}
}

func rebootMessage(t time.Time) {
	deck.InfoA("Updates have been installed, please reboot to complete the installation...").With(eventID(cablib.EvtInstallSuccess)).Go()

	if err := notification.NewRebootMessage(t).Push(); err != nil {
		deck.ErrorfA("Failed to create notification:\n%v", err).With(eventID(cablib.EvtErrNotifications)).Go()
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

func installCollection(s *session.UpdateSession, c *updatecollection.Collection, ipu bool) (*installRsp, error) {
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
	if err != nil {
		return nil, fmt.Errorf("error getting install RebootRequired:\n %v", err)
	}

	if ipu {
		if err := inst.Commit(); err != nil {
			return nil, fmt.Errorf("error committing updates:\n %v", err)
		}
	}

	return &installRsp{
		hResult:        hr,
		resultCode:     rc,
		rebootRequired: rb,
	}, err
}

func (i *installCmd) installUpdates() error {
	// If monthly patches are disabled, and no specific update type was requested, do nothing.
	if config.InstallMonthlyPatches == 0 && !i.all && !i.drivers && !i.virusDef && i.kbs == "" {
		deck.InfoA("InstallMonthlyPatches is disabled, skipping default update installation.").With(eventID(cablib.EvtMisc)).Go()
		return nil
	}
	// Check for reboot status when not installing virus definitions.
	if !(i.virusDef) {
		rebootRequired, err := cablib.RebootRequired()
		if err != nil {
			return fmt.Errorf("failed to determine reboot status: %v", err)
		}

		if rebootRequired {
			if i.Interactive {
				fmt.Println("Host has existing updates pending reboot.")
				rebootEvent <- rebootRequired
				return nil
			}
			t, err := cablib.RebootTime()
			if err != nil {
				return fmt.Errorf("Error getting reboot time: %v", err)
			}
			if t.IsZero() {
				// Don't trigger a reboot if one is pending but no time has been set.
				// This can happen when updates are installed outside of Cabbie.
				//
				// TODO(b/402737358): Consider reverting this once installs outside of
				// maintenance windows are root-caused for causing daily reboots.
				return nil
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
		deck.ErrorfA("Error posting metric:\n%v", er).With(eventID(cablib.EvtErrMetricReport)).Go()
	}
	if err != nil {
		return fmt.Errorf("error encountered when attempting to query for updates: %v", err)
	}
	defer uc.Close()

	if len(uc.Updates) == 0 {
		deck.InfoA("No updates found to install.").With(eventID(cablib.EvtNoUpdates)).Go()
		return nil
	}
	deck.InfofA("Updates Found:\n%s", strings.Join(uc.Titles(), "\n\n")).With(eventID(cablib.EvtUpdatesFound)).Go()

	installMsgPopped := i.virusDef
	installingMinOneUpdate := false

	kbs := NewKBSet(i.kbs)
	if err := initDriverExclusion(); err != nil {
		deck.ErrorfA("Error initializing driver exclusions:\n%v", err).With(eventID(cablib.EvtErrDriverExclusion)).Go()
	}
	excludes := excludedDrivers.get()
outerLoop:
	for _, u := range uc.Updates {
		for _, e := range excludes {
			t := time.Time{}
			if e.DriverDateVer != "" {
				t, err = time.Parse("2006-01-02", e.DriverDateVer)
				if err != nil {
					deck.WarningfA("Failed to parse driver date version provided in exclusion json: %v", err).With(eventID(cablib.EvtErrDriverExclusion)).Go()
				}
			}
			// Check if at least one driver exclusion exists and matches the update being evaluated.
			driverFilterExists := e.DriverClass != "" || !t.IsZero()
			driverClassMatch := e.DriverClass == "" || e.DriverClass == u.DriverClass
			driverVersionMatch := t.IsZero() || t.Equal(u.DriverVerDate)
			if driverFilterExists && driverClassMatch && driverVersionMatch {
				deck.InfofA(
					"Driver update %q excluded.\nFiltered driver class: %q\nFiltered driver date version: %q",
					u.Title, e.DriverClass, e.DriverDateVer,
				).With(eventID(cablib.EvtDriverUpdateExcluded)).Go()
				continue outerLoop
			}
		}
		if !(u.InCategories(rc)) {
			deck.InfofA("Skipping update %s.\nRequiredClassifications:\n%v\nUpdate classifications:\n%v",
				u.Title,
				rc,
				u.Categories).With(eventID(cablib.EvtUpdateSkip)).Go()
			continue
		}

		if !(u.EulaAccepted) {
			deck.InfofA("Accepting EULA for update: %s", u.Title).With(eventID(cablib.EvtMisc)).Go()
			if err := u.AcceptEula(); err != nil {
				deck.ErrorfA("Failed to accept EULA for update %s:\n%s", u.Title, err).With(eventID(cablib.EvtErrMisc)).Go()
			}
		}

		if kbs.Size() > 0 {
			if !kbs.Search(u.KBArticleIDs) {
				deck.InfofA("Skipping update %s.\nRequired KBs:\n%s\nUpdate KBs:\n%v",
					u.Title,
					kbs,
					u.KBArticleIDs).With(eventID(cablib.EvtUpdateSkip)).Go()
				continue
			}
		}
		if i.deadlineOnly {
			deadline := time.Duration(config.Deadline) * 24 * time.Hour
			pastDeadline := time.Now().After(u.LastDeploymentChangeTime.Add(deadline))
			if u.DriverClass != "" {
				deck.InfofA(
					"Skipping driver %s with class %s and date version %s.\nDrivers are only installed during a maintenance window at this time.",
					u.Title,
					u.DriverClass,
					u.DriverVerDate).With(eventID(cablib.EvtUpdateSkip)).Go()
				continue
			}
			if !pastDeadline {
				deck.InfofA(
					"Skipping update %s.\nUpdate deployed on %v has not reached the %d day threshold.",
					u.Title,
					u.LastDeploymentChangeTime,
					config.Deadline).With(eventID(cablib.EvtUpdateSkip)).Go()
				continue
			}
			deck.InfofA(
				"Update %s deployed on %v has exceeded the %d day threshold.",
				u.Title,
				u.LastDeploymentChangeTime,
				config.Deadline).With(eventID(cablib.EvtUpdatesFound)).Go()
		}

		c, err := updatecollection.New()
		if err != nil {
			deck.ErrorfA("Failed to create collection: %v", err).With(eventID(cablib.EvtErrMisc)).Go()
			continue
		}
		c.Add(u.Item)

		if !installMsgPopped && !u.InCategories([]string{"Definition Updates"}) {
			installingMessage()
			installMsgPopped = true
			ps := filepath.Join(cablib.CabbiePath, "PreUpdate.ps1")
			exist, err := helpers.PathExists(ps)
			if err != nil {
				deck.ErrorfA("PreUpdateScript: error checking existence of %q:\n%v", cablib.CabbiePath+"PreUpdate.ps1", err).With(eventID(cablib.EvtErrUpdateScript)).Go()
			} else if exist {
				if _, err := helpers.ExecWithVerify(ps, nil, &config.ScriptTimeout, nil); err != nil {
					deck.ErrorfA("PreUpdateScript: error running script:\n%v", err).With(eventID(cablib.EvtErrUpdateScript)).Go()
				}
			}
			installingMinOneUpdate = true
		}

		deck.InfofA("Downloading Update:\n%v", u).With(eventID(cablib.EvtDownload)).Go()

		rc, err := downloadCollection(s, c)
		if err != nil {
			deck.ErrorA(err).With(eventID(cablib.EvtErrMisc)).Go()
			c.Close()
			continue
		}
		if rc == 2 {
			deck.InfofA("Successfully downloaded update:\n %s", u.Title).With(eventID(cablib.EvtDownload)).Go()
		} else {

			deck.ErrorfA("Failed to download update:\n %s\n ReturnCode: %d", u.Title, rc).With(eventID(cablib.EvtErrDownloadFailure)).Go()
			c.Close()
			continue
		}

		deck.InfofA("Installing Update:\n%v", u).With(eventID(cablib.EvtInstall)).Go()

		ipu := false
		if u.InCategories([]string{"Upgrades"}) {
			ipu = true
		}

		rsp, err := installCollection(s, c, ipu)
		if err != nil {
			deck.ErrorA(err).With(eventID(cablib.EvtErrMisc)).Go()
			c.Close()
			continue
		}

		if err := installHResult.Set(rsp.hResult); err != nil {
			deck.ErrorfA("Error posting metric:\n%v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
		if rsp.resultCode == 2 {
			deck.InfofA("Successfully installed update:\n%s\nHResult Code: %s", u.Title, rsp.hResult).With(eventID(cablib.EvtInstall)).Go()
		} else {
			deck.ErrorfA("Failed to install update:\n%s\nReturnCode: %d\nHResult Code: %s", u.Title, rsp.resultCode, rsp.hResult).With(eventID(cablib.EvtErrInstallFailure)).Go()
			c.Close()
			continue
		}

		deck.InfofA("Install of KB %s; Reboot Required: %t", u.KBArticleIDs, rsp.rebootRequired).With(eventID(cablib.EvtRebootRequired)).Go()

		if rsp.rebootRequired && !u.InCategories([]string{"Definition Updates"}) {
			deck.InfofA("Adding KB %s to reboot list.", u.KBArticleIDs).With(eventID(cablib.EvtRebootRequired)).Go()
			rebootList = append(rebootList, u.KBArticleIDs...)
		}

		if rsp.rebootRequired && u.InCategories([]string{"Upgrades"}) {
			if err := cablib.SetInstallAtShutdown(); err != nil {
				deck.ErrorfA("Failed to set `InstallAtShutdown` registry value: %v", err).With(eventID(cablib.EvtErrPowerMgmt)).Go()
			}
		}

		c.Close()
	}

	if installingMinOneUpdate {
		ps := filepath.Join(cablib.CabbiePath, "PostUpdate.ps1")
		exist, err := helpers.PathExists(ps)
		if err != nil {
			deck.ErrorfA("PostUpdateScript: error checking existence of %q:\n%v", cablib.CabbiePath+"PostUpdate.ps1", err).With(eventID(cablib.EvtErrUpdateScript)).Go()
		} else if exist {
			if _, err := helpers.ExecWithVerify(ps, nil, &config.ScriptTimeout, nil); err != nil {
				deck.ErrorfA("PostUpdateScript: error executing script:\n%v", err).With(eventID(cablib.EvtErrUpdateScript)).Go()
			}
		}
	}

	if len(rebootList) > 0 {
		if err := cablib.AddRebootUpdates(rebootList); err != nil {
			deck.ErrorfA("Failed to write updates requiring reboot to registry: %v", err).With(eventID(cablib.EvtRebootRequired)).Go()
		}

		// Use active hours if enabled and available, otherwise use the standard reboot delay.
		now := time.Now()
		timerEnd := now.Add(time.Second * time.Duration(config.RebootDelay))
		rebootTime := timerEnd
		if config.ActiveHoursEnabled == 1 {
			ah, err := client.Label(int(config.AukeraPort), `active_hours`)
			if err != nil {
				deck.ErrorfA("Error getting maintenance window %q with error:\n%v", `active_hours`, err).With(eventID(cablib.EvtErrMaintWindow)).Go()
			}
			if len(ah) != 0 {
				todayEnd := ah[0].Closes
				tomorrowEnd := todayEnd.Add(time.Hour * time.Duration(24))
				// If the active hours end time is in the future, use the end time.
				// Otherwise, use the same end time of the next day.
				if todayEnd.After(now) {
					rebootTime = todayEnd
				} else {
					rebootTime = tomorrowEnd
				}
			}
		}
		rebootMessage(rebootTime)
		if err := cablib.SetRebootTime(rebootTime); err != nil {
			deck.ErrorfA("Failed to set reboot time:\n%v", err).With(eventID(cablib.EvtErrPowerMgmt)).Go()
		}
		rebootEvent <- true
	}

	return nil
}
