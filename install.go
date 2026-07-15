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
	"bytes"
	"golang.org/x/net/context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"flag"
	"github.com/google/cabbie/notification"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/download"
	"github.com/google/cabbie/enforcement"
	"github.com/google/cabbie/install"
	"github.com/google/cabbie/search"
	"github.com/google/cabbie/session"
	"github.com/google/cabbie/updatecollection"
	"github.com/google/cabbie/updates"
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

func (i installCmd) Execute(ctx context.Context, flags *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	if err := vetFlags(i); err != nil {
		return subcommands.ExitUsageError
	}

	if err := i.installUpdates(ctx); err != nil {
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

	if err := notification.NewInstallingMessage().Push(context.Background()); err != nil {
		deck.ErrorfA("Failed to create notification:\n%v", err).With(eventID(cablib.EvtErrNotifications)).Go()
	}
}

func rebootMessage(t time.Time) {
	deck.InfoA("Updates have been installed, please reboot to complete the installation...").With(eventID(cablib.EvtInstallSuccess)).Go()

	if err := notification.NewRebootMessage(t).Push(context.Background()); err != nil {
		deck.ErrorfA("Failed to create notification:\n%v", err).With(eventID(cablib.EvtErrNotifications)).Go()
	}
}

func downloadCollection(s *session.UpdateSession, c *updatecollection.Collection) (int, error) {
	if s == nil {
		return 0, errors.New("update session is nil")
	}
	if c == nil {
		return 0, errors.New("update collection is nil")
	}
	d, err := download.NewDownloader(s, c)
	if err != nil {
		return 0, fmt.Errorf("error creating downloader:\n %v", err)
	}
	defer d.Close()

	if err := d.Download(context.Background()); err != nil {
		return 0, fmt.Errorf("error downloading updates:\n %v", err)
	}

	return d.ResultCode()
}

// fetchDetailedUpdateError queries the Windows Event Log for recent update installation
// failures matching the given title and returns any specific error code found in
// the event message and true, or empty string and false if not found or on error.
func fetchDetailedUpdateError(ctx context.Context, title string) (string, bool) {
	// Query for event ID 20 from Microsoft-Windows-WindowsUpdateClient in the System log
	// within the last 5 minutes. Filter messages that contain the update title.
	// Sort by newest first, take the first result, and extract the first
	// hexadecimal code found in the message, if any.
	psTitle := "'" + strings.ReplaceAll(title, "'", "''") + "'"
	psCmd := fmt.Sprintf(`Get-WinEvent -FilterHashtable @{LogName='System';ProviderName='Microsoft-Windows-WindowsUpdateClient';Id=20;StartTime=(Get-Date).AddMinutes(-5)} -ErrorAction SilentlyContinue |Where-Object {$_.Message -match [regex]::Escape(%s)} |Sort-Object TimeCreated -Descending |Select-Object -First 1 |ForEach-Object { if($_.Message -match '(0x[0-9a-fA-F]+)'){ $matches[1] } }`, psTitle)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, "-NoProfile", "-Command", psCmd)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		deck.WarningfA("Failed to execute PowerShell to get detailed update error for %q: %v, stderr: %q", title, err, stderr.String()).Go()
		return "", false
	}
	code := strings.TrimSpace(out.String())
	if code == "" {
		return "", false
	}
	return code, true
}

func installCollection(s *session.UpdateSession, c *updatecollection.Collection, ipu bool) (*installRsp, error) {
	if s == nil {
		return nil, errors.New("update session is nil")
	}
	if c == nil {
		return nil, errors.New("update collection is nil")
	}
	inst, err := install.NewInstaller(s, c)
	if err != nil {
		return nil, fmt.Errorf("error creating installer: \n %v", err)
	}
	defer inst.Close()

	if err := inst.Install(context.Background()); err != nil {
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
		if err := inst.Commit(context.Background()); err != nil {
			return nil, fmt.Errorf("error committing updates:\n %v", err)
		}
	}

	return &installRsp{
		hResult:        hr,
		resultCode:     rc,
		rebootRequired: rb,
	}, err
}

func checkPendingReboot(interactive bool) (bool, error) {
	rebootRequired, err := cablib.RebootRequired()
	if err != nil {
		return false, fmt.Errorf("failed to determine reboot status: %v", err)
	}

	if rebootRequired {
		if interactive {
			fmt.Println("Host has existing updates pending reboot.")
			rebootEvent <- rebootRequired
			return true, nil
		}
		t, err := cablib.RebootTime()
		if err != nil {
			return false, fmt.Errorf("Error getting reboot time: %v", err)
		}
		if t.IsZero() {
			return true, nil
		}
		rebootEvent <- rebootRequired
		return true, nil
	}
	return false, nil
}

func isDriverExcluded(u *updates.Update, excludes []enforcement.DriverExclude) bool {
	if u == nil {
		return false
	}
	for _, e := range excludes {
		t := time.Time{}
		if e.DriverDateVer != "" {
			var err error
			t, err = time.Parse("2006-01-02", e.DriverDateVer)
			if err != nil {
				deck.WarningfA("Failed to parse driver date version provided in exclusion json: %v", err).With(eventID(cablib.EvtErrDriverExclusion)).Go()
			}
		}
		driverFilterExists := e.DriverClass != "" || !t.IsZero()
		driverClassMatch := e.DriverClass == "" || e.DriverClass == u.DriverClass
		driverVersionMatch := t.IsZero() || t.Equal(u.DriverVerDate)
		if driverFilterExists && driverClassMatch && driverVersionMatch {
			deck.InfofA(
				"Driver update %q excluded.\nFiltered driver class: %q\nFiltered driver date version: %q",
				u.Title, e.DriverClass, e.DriverDateVer,
			).With(eventID(cablib.EvtDriverUpdateExcluded)).Go()
			return true
		}
	}
	return false
}

func (i *installCmd) shouldSkipUpdate(u *updates.Update, rc []string, kbs KBSet) bool {
	if u == nil {
		return true
	}
	if !(u.InCategories(rc)) {
		deck.InfofA("Skipping update %s.\nRequiredClassifications:\n%v\nUpdate classifications:\n%v",
			u.Title, rc, u.Categories).With(eventID(cablib.EvtUpdateSkip)).Go()
		return true
	}

	if kbs.Size() > 0 && !kbs.Search(u.KBArticleIDs) {
		deck.InfofA("Skipping update %s.\nRequired KBs:\n%s\nUpdate KBs:\n%v",
			u.Title, kbs, u.KBArticleIDs).With(eventID(cablib.EvtUpdateSkip)).Go()
		return true
	}

	if i.deadlineOnly {
		deadline := time.Duration(config.Deadline) * 24 * time.Hour
		pastDeadline := time.Now().After(u.LastDeploymentChangeTime.Add(deadline))
		if u.DriverClass != "" {
			deck.InfofA(
				"Skipping driver %s with class %s and date version %s.\nDrivers are only installed during a maintenance window at this time.",
				u.Title, u.DriverClass, u.DriverVerDate).With(eventID(cablib.EvtUpdateSkip)).Go()
			return true
		}
		if !pastDeadline {
			deck.InfofA(
				"Skipping update %s.\nUpdate deployed on %v has not reached the %d day threshold.",
				u.Title, u.LastDeploymentChangeTime, config.Deadline).With(eventID(cablib.EvtUpdateSkip)).Go()
			return true
		}
		deck.InfofA(
			"Update %s deployed on %v has exceeded the %d day threshold.",
			u.Title, u.LastDeploymentChangeTime, config.Deadline).With(eventID(cablib.EvtUpdatesFound)).Go()
	}

	return false
}

func runScript(scriptName, scriptType string) {
	ps := filepath.Join(cablib.CabbiePath, scriptName)
	exist, err := helpers.PathExists(ps)
	if err != nil {
		deck.ErrorfA("%s: error checking existence of %q:\n%v", scriptType, ps, err).With(eventID(cablib.EvtErrUpdateScript)).Go()
	} else if exist {
		if _, err := helpers.ExecWithVerify(ps, nil, &config.ScriptTimeout, nil); err != nil {
			deck.ErrorfA("%s: error running script:\n%v", scriptType, err).With(eventID(cablib.EvtErrUpdateScript)).Go()
		}
	}
}

func scheduleReboot(rebootList []string) {
	if len(rebootList) > 0 {
		if err := cablib.AddRebootUpdates(rebootList); err != nil {
			deck.ErrorfA("Failed to write updates requiring reboot to registry: %v", err).With(eventID(cablib.EvtRebootRequired)).Go()
		}
	}

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

// UpdateOptions holds options and criteria for update operations.
type UpdateOptions struct {
	RequiredCategories []string
	KBSet              KBSet
	DriverExcludes     []enforcement.DriverExclude
	DeadlineOnly       bool
}

// UpdateResult contains the result of processing a single update.
type UpdateResult struct {
	Skipped        bool
	Downloaded     bool
	Installed      bool
	RebootRequired bool
	KBArticleIDs   []string
	Err            error
}

// UpdateRunner orchestrates downloading and installing updates while tracking reboot and notification state.
type UpdateRunner struct {
	ctx     context.Context
	session *session.UpdateSession
	cmd     *installCmd
	options UpdateOptions

	installMsgPopped bool
	installedAny     bool
	anyReboot        bool
	rebootList       []string
}

// NewUpdateRunner creates a new UpdateRunner instance.
func NewUpdateRunner(ctx context.Context, s *session.UpdateSession, cmd *installCmd, opts UpdateOptions) *UpdateRunner {
	initialMsgPopped := false
	if cmd != nil {
		initialMsgPopped = cmd.virusDef
	}
	return &UpdateRunner{
		ctx:              ctx,
		session:          s,
		cmd:              cmd,
		options:          opts,
		installMsgPopped: initialMsgPopped,
	}
}

func (r *UpdateRunner) shouldSkip(u *updates.Update) bool {
	if u == nil {
		return true
	}
	if isDriverExcluded(u, r.options.DriverExcludes) {
		return true
	}
	if r.cmd != nil {
		return r.cmd.shouldSkipUpdate(u, r.options.RequiredCategories, r.options.KBSet)
	}
	return false
}

func (r *UpdateRunner) ensureEulaAccepted(u *updates.Update) {
	if u == nil {
		return
	}
	if !(u.EulaAccepted) {
		deck.InfofA("Accepting EULA for update: %s", u.Title).With(eventID(cablib.EvtMisc)).Go()
		if err := u.AcceptEula(); err != nil {
			deck.ErrorfA("Failed to accept EULA for update %s:\n%s", u.Title, err).With(eventID(cablib.EvtErrMisc)).Go()
		}
	}
}

func (r *UpdateRunner) triggerPreUpdateHooks(u *updates.Update) {
	if u == nil {
		return
	}
	if !r.installMsgPopped && !u.InCategories([]string{"Definition Updates"}) {
		installingMessage()
		r.installMsgPopped = true
		runScript("PreUpdate.ps1", "PreUpdateScript")
		r.installedAny = true
	}
}

func (r *UpdateRunner) downloadSingleUpdate(u *updates.Update, c *updatecollection.Collection) error {
	if u == nil {
		return errors.New("cannot download nil update")
	}
	if c == nil {
		return errors.New("cannot download nil collection")
	}
	if r == nil || r.session == nil {
		return errors.New("cannot download with nil session")
	}
	deck.InfofA("Downloading Update:\n%v", u).With(eventID(cablib.EvtDownload)).Go()

	dlRc, err := downloadCollection(r.session, c)
	if err != nil {
		deck.ErrorA(err).With(eventID(cablib.EvtErrMisc)).Go()
		return err
	}
	if dlRc == 2 {
		deck.InfofA("Successfully downloaded update:\n %s", u.Title).With(eventID(cablib.EvtDownload)).Go()
		return nil
	}
	err = fmt.Errorf("failed to download update: %s, ReturnCode: %d", u.Title, dlRc)
	deck.ErrorfA("Failed to download update:\n %s\n ReturnCode: %d", u.Title, dlRc).With(eventID(cablib.EvtErrDownloadFailure)).Go()
	return err
}

func (r *UpdateRunner) installSingleUpdate(u *updates.Update, c *updatecollection.Collection) (*installRsp, error) {
	if u == nil {
		return nil, errors.New("cannot install nil update")
	}
	if c == nil {
		return nil, errors.New("cannot install nil collection")
	}
	if r == nil || r.session == nil {
		return nil, errors.New("cannot install with nil session")
	}
	deck.InfofA("Installing Update:\n%v", u).With(eventID(cablib.EvtInstall)).Go()

	ipu := u.InCategories([]string{"Upgrades"})

	rsp, err := installCollection(r.session, c, ipu)
	if err != nil {
		deck.ErrorA(err).With(eventID(cablib.EvtErrMisc)).Go()
		return nil, err
	}

	if installHResult != nil {
		if err := installHResult.Set(rsp.hResult); err != nil {
			deck.ErrorfA("Error posting metric:\n%v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}
	if rsp.resultCode == 2 {
		deck.InfofA("Successfully installed update:\n%s\nHResult Code: %s", u.Title, rsp.hResult).With(eventID(cablib.EvtInstall)).Go()
		return rsp, nil
	}

	deck.ErrorfA("Failed to install update:\n%s\nReturnCode: %d\nHResult Code: %s", u.Title, rsp.resultCode, rsp.hResult).With(eventID(cablib.EvtErrInstallFailure)).Go()
	if r != nil && r.ctx != nil {
		if code, ok := fetchDetailedUpdateError(r.ctx, u.Title); ok {
			deck.WarningfA("Detailed error for update %q from Windows Update Client log: %s", u.Title, code).Go()
		}
	}
	return rsp, fmt.Errorf("install failed with resultCode %d", rsp.resultCode)
}

func (r *UpdateRunner) recordRebootState(u *updates.Update, rsp *installRsp) {
	if u == nil || rsp == nil {
		return
	}
	deck.InfofA("Install of KB %s; Reboot Required: %t", u.KBArticleIDs, rsp.rebootRequired).With(eventID(cablib.EvtRebootRequired)).Go()

	if rsp.rebootRequired && !u.InCategories([]string{"Definition Updates"}) {
		r.anyReboot = true
		deck.InfofA("Adding KB %s to reboot list.", u.KBArticleIDs).With(eventID(cablib.EvtRebootRequired)).Go()
		r.rebootList = append(r.rebootList, u.KBArticleIDs...)
	}

	if rsp.rebootRequired && u.InCategories([]string{"Upgrades"}) {
		if err := cablib.SetInstallAtShutdown(); err != nil {
			deck.ErrorfA("Failed to set `InstallAtShutdown` registry value: %v", err).With(eventID(cablib.EvtErrPowerMgmt)).Go()
		}
	}
}

// ProcessSingleUpdate handles downloading and installing a single update item.
func (r *UpdateRunner) ProcessSingleUpdate(u *updates.Update) UpdateResult {
	if u == nil {
		return UpdateResult{Skipped: true, Err: errors.New("cannot process nil update")}
	}
	if r.shouldSkip(u) {
		return UpdateResult{Skipped: true}
	}

	r.ensureEulaAccepted(u)

	c, err := updatecollection.New()
	if err != nil {
		deck.ErrorfA("Failed to create collection: %v", err).With(eventID(cablib.EvtErrMisc)).Go()
		return UpdateResult{Err: err}
	}
	defer c.Close()

	if err := c.Add(u.Item); err != nil {
		deck.ErrorfA("Failed to add update to collection: %v", err).With(eventID(cablib.EvtErrMisc)).Go()
		return UpdateResult{Err: err}
	}

	r.triggerPreUpdateHooks(u)

	if err := r.downloadSingleUpdate(u, c); err != nil {
		return UpdateResult{Err: err}
	}

	rsp, err := r.installSingleUpdate(u, c)
	if err != nil {
		return UpdateResult{Downloaded: true, Err: err}
	}

	r.recordRebootState(u, rsp)

	return UpdateResult{
		Downloaded:     true,
		Installed:      true,
		RebootRequired: rsp.rebootRequired,
		KBArticleIDs:   u.KBArticleIDs,
	}
}

// Finalize executes post-update scripts and schedules reboots if required.
func (r *UpdateRunner) Finalize() {
	if r.installedAny {
		runScript("PostUpdate.ps1", "PostUpdateScript")
	}

	if r.anyReboot || len(r.rebootList) > 0 {
		scheduleReboot(r.rebootList)
	}
}

func (i *installCmd) processSingleUpdate(
	ctx context.Context,
	s *session.UpdateSession,
	u *updates.Update,
	rc []string,
	kbs KBSet,
	excludes []enforcement.DriverExclude,
	installMsgPopped *bool,
	installingMinOneUpdate *bool,
	anyRebootRequired *bool,
	rebootList *[]string,
) {
	runner := &UpdateRunner{
		ctx:              ctx,
		session:          s,
		cmd:              i,
		options:          UpdateOptions{RequiredCategories: rc, KBSet: kbs, DriverExcludes: excludes, DeadlineOnly: i.deadlineOnly},
		installMsgPopped: *installMsgPopped,
		installedAny:     *installingMinOneUpdate,
		anyReboot:        *anyRebootRequired,
		rebootList:       *rebootList,
	}
	res := runner.ProcessSingleUpdate(u)
	*installMsgPopped = runner.installMsgPopped
	*installingMinOneUpdate = runner.installedAny
	*anyRebootRequired = runner.anyReboot
	*rebootList = runner.rebootList
	_ = res
}

func (i *installCmd) installUpdates(ctx context.Context) error {
	// If monthly patches are disabled, and no specific update type was requested, do nothing.
	if config.InstallMonthlyPatches == 0 && !i.all && !i.drivers && !i.virusDef && i.kbs == "" {
		deck.InfoA("InstallMonthlyPatches is disabled, skipping default update installation.").With(eventID(cablib.EvtMisc)).Go()
		return nil
	}
	// Check for reboot status when not installing virus definitions.
	if !i.virusDef {
		pending, err := checkPendingReboot(i.Interactive)
		if err != nil || pending {
			return err
		}
	}

	// Start Windows update session
	s, err := session.New()
	if err != nil {
		return fmt.Errorf("failed to create new Windows Update session: %v", err)
	}
	defer s.Close()

	criteria, rc := i.criteria()

	uc, hResult, err := search.FindUpdates(s, criteria, config.WSUSServers, config.EnableThirdParty)
	if searchHResult != nil {
		if er := searchHResult.Set(hResult); er != nil {
			deck.ErrorfA("Error posting metric:\n%v", er).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}
	if err != nil {
		return err
	}
	defer uc.Close()

	if len(uc.Updates) == 0 {
		deck.InfoA("No updates found to install.").With(eventID(cablib.EvtNoUpdates)).Go()
		return nil
	}
	deck.InfofA("Updates Found:\n%s", strings.Join(uc.Titles(), "\n\n")).With(eventID(cablib.EvtUpdatesFound)).Go()

	kbs := NewKBSet(i.kbs)
	if err := initDriverExclusion(); err != nil {
		deck.ErrorfA("Error initializing driver exclusions:\n%v", err).With(eventID(cablib.EvtErrDriverExclusion)).Go()
	}
	excludes := excludedDrivers.get()

	options := UpdateOptions{
		RequiredCategories: rc,
		KBSet:              kbs,
		DriverExcludes:     excludes,
		DeadlineOnly:       i.deadlineOnly,
	}

	runner := NewUpdateRunner(ctx, s, i, options)

	for _, u := range uc.Updates {
		runner.ProcessSingleUpdate(u)
	}

	runner.Finalize()

	return nil
}
