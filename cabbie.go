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

//go:build windows
// +build windows

// The cabbie binary is used to manage and report Windows updates.
package main

import (
	"golang.org/x/net/context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"

	"flag"
	"github.com/google/cabbie/metrics"
	"github.com/google/cabbie/notification"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/enforcement"
	"github.com/google/cabbie/servicemgr"
	"github.com/google/deck/backends/eventlog"
	"github.com/google/deck/backends/logger"
	"github.com/google/deck"
	"github.com/google/aukera/client"
	"github.com/scjalliance/comshim"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc"
	"github.com/google/subcommands"
)

var (
	runInDebug        = flag.Bool("debug", false, "Run in debug mode")
	verbose           = flag.Bool("verbose", false, "Enable verbose (stdout) logging")
	runNormalPriority = flag.Bool("normalpriority", false, "If specified cabbie runs at normal priority rather than lowering the process priority")
	config            = new(Settings)
	categoryDefaults  = []string{"Critical Updates", "Definition Updates", "Security Updates"}
	rebootEvent       = make(chan bool, 10)
	rebootActive      atomic.Bool

	excludedDrivers driverExcludes

	// Metrics
	virusUpdateSuccess         *metrics.Bool
	listUpdateSuccess          *metrics.Bool
	driverUpdateSuccess        *metrics.Bool
	updateInstallSuccess       *metrics.Bool
	rebootRequired             *metrics.Bool
	deviceIsPatched            *metrics.Bool
	requiredUpdateCount        *metrics.Int
	enforcedUpdateCount        *metrics.Int
	enforcementWatcherFailures *metrics.Int
	installHResult             *metrics.String
	searchHResult              *metrics.String

	eventID = eventlog.EventID
)

func isRebootActive() bool {
	return rebootActive.Load()
}

func setRebootActive(v bool) {
	rebootActive.Store(v)
}

// Settings contains configurable options.
type Settings struct {
	WSUSServers, RequiredCategories                                                                                       []string
	InstallDrivers, InstallVirusDefs, EnableThirdParty, RebootDelay, Deadline, EnableNotifications, InstallMonthlyPatches uint64

	// Aukera Integration
	AukeraEnabled uint64
	AukeraPort    uint64
	AukeraName    string

	// Microsoft Active Hours Integration (Requires Aukera Enabled)
	ActiveHoursEnabled uint64

	PprofPort uint64

	ScriptTimeout time.Duration
}

type tickers struct {
	Default, Aukera, List, Virus, Driver, Enforcement *time.Ticker
}

type driverExcludes struct {
	mutex sync.RWMutex
	e     []enforcement.DriverExclude
}

func (d *driverExcludes) set(v []enforcement.DriverExclude) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.e = v
}

func (d *driverExcludes) get() []enforcement.DriverExclude {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.e
}

func initTickers() tickers {
	return tickers{
		Default:     time.NewTicker(24 * time.Hour),
		Aukera:      time.NewTicker(5 * time.Minute),
		List:        time.NewTicker(2 * time.Hour),
		Virus:       time.NewTicker(30 * time.Minute),
		Driver:      time.NewTicker(72 * time.Hour),
		Enforcement: time.NewTicker(6 * time.Hour),
	}
}

func (t *tickers) stop() {
	t.Default.Stop()
	t.Aukera.Stop()
	t.List.Stop()
	t.Virus.Stop()
	t.Driver.Stop()
	t.Enforcement.Stop()
}

func newSettings() *Settings {
	// Set non-Zero defaults.
	return &Settings{
		AukeraName:            cablib.SvcName,
		RequiredCategories:    categoryDefaults,
		InstallVirusDefs:      1,
		InstallMonthlyPatches: 1,
		RebootDelay:           21600,
		Deadline:              14,
		EnableNotifications:   1,
		AukeraPort:            9119,
		ScriptTimeout:         10 * time.Minute,
	}
}

func readInt(k registry.Key, name string, dest *uint64) {
	if i, _, err := k.GetIntegerValue(name); err == nil {
		*dest = i
	}
}

func readIntWithDeprecatedFallback(k registry.Key, name, deprecatedName string, dest *uint64) {
	if i, _, err := k.GetIntegerValue(name); err == nil {
		*dest = i
	} else if i, _, err := k.GetIntegerValue(deprecatedName); err == nil {
		*dest = i
		deck.WarningfA("Registry setting %q is deprecated and will be removed in a future version. Please use %q instead.", deprecatedName, name).With(eventID(cablib.EvtErrConfig)).Go()
	}
}

func (s *Settings) regLoad(path string) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if m, _, err := k.GetStringsValue("WsusServers"); err == nil {
		s.WSUSServers = m
	}

	if a, _, err := k.GetStringValue("AukeraName"); err == nil {
		s.AukeraName = a
	} else {
		deck.InfofA("AukeraName not found in registry, using default Name:\n%v", s.AukeraName).With(eventID(cablib.EvtErrConfig)).Go()
	}

	if m, _, err := k.GetStringsValue("RequiredCategories"); err == nil {
		s.RequiredCategories = m
	} else {
		deck.InfofA("RequiredCategories not found in registry, using default categories:\n%v", s.RequiredCategories).With(eventID(cablib.EvtErrConfig)).Go()
	}

	readInt(k, "EnableThirdParty", &s.EnableThirdParty)
	readIntWithDeprecatedFallback(k, "InstallDrivers", "UpdateDrivers", &s.InstallDrivers)
	readIntWithDeprecatedFallback(k, "InstallVirusDefs", "UpdateVirusDef", &s.InstallVirusDefs)
	readInt(k, "RebootDelay", &s.RebootDelay)
	readInt(k, "Deadline", &s.Deadline)
	readIntWithDeprecatedFallback(k, "EnableNotifications", "NotifyAvailable", &s.EnableNotifications)
	readInt(k, "AukeraEnabled", &s.AukeraEnabled)
	readInt(k, "AukeraPort", &s.AukeraPort)
	readInt(k, "PprofPort", &s.PprofPort)
	readInt(k, "ActiveHoursEnabled", &s.ActiveHoursEnabled)
	readInt(k, "InstallMonthlyPatches", &s.InstallMonthlyPatches)

	if i, _, err := k.GetIntegerValue("ScriptTimeout"); err == nil {
		s.ScriptTimeout = time.Duration(i) * time.Minute
	}

	return nil
}

// Type winSvc implements svc.Handler.
type winSvc struct{}

func startService(isDebug bool) error {
	deck.InfofA("Starting %s service.", cablib.SvcName).With(eventID(cablib.EvtServiceStarting)).Go()
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	if err := run(cablib.SvcName, winSvc{}); err != nil {
		return fmt.Errorf("%s service failed. %v", cablib.SvcName, err)
	}
	deck.InfofA("%s service stopped.", cablib.SvcName).With(eventID(cablib.EvtServiceStopped)).Go()
	return nil
}

func initMetrics() error {
	var firstErr error
	initMetric := func(name string, fn func() error) {
		if firstErr != nil {
			return
		}
		if err := fn(); err != nil {
			firstErr = fmt.Errorf("unable to initialize %s metric: %v", name, err)
		}
	}

	initMetric("virusUpdateSuccess", func() (e error) { virusUpdateSuccess, e = metrics.NewBool(cablib.MetricRoot+"virusUpdateSuccess", cablib.MetricSvc); return })
	initMetric("listUpdateSuccess", func() (e error) { listUpdateSuccess, e = metrics.NewBool(cablib.MetricRoot+"listUpdateSuccess", cablib.MetricSvc); return })
	initMetric("driverUpdateSuccess", func() (e error) { driverUpdateSuccess, e = metrics.NewBool(cablib.MetricRoot+"driverUpdateSuccess", cablib.MetricSvc); return })
	initMetric("updateInstallSuccess", func() (e error) { updateInstallSuccess, e = metrics.NewBool(cablib.MetricRoot+"updateInstallSuccess", cablib.MetricSvc); return })
	initMetric("rebootRequired", func() (e error) { rebootRequired, e = metrics.NewBool(cablib.MetricRoot+"rebootRequired", cablib.MetricSvc); return })
	initMetric("deviceIsPatched", func() (e error) { deviceIsPatched, e = metrics.NewBool(cablib.MetricRoot+"deviceIsPatched", cablib.MetricSvc); return })

	initMetric("requiredUpdateCount", func() (e error) { requiredUpdateCount, e = metrics.NewInt(cablib.MetricRoot+"requiredUpdateCount", cablib.MetricSvc); return })
	initMetric("enforcedUpdateCount", func() (e error) { enforcedUpdateCount, e = metrics.NewInt(cablib.MetricRoot+"enforcedUpdateCount", cablib.MetricSvc); return })
	initMetric("enforcementWatcherFailures", func() (e error) { enforcementWatcherFailures, e = metrics.NewCounter(cablib.MetricRoot+"enforcementWatcherFailures", cablib.MetricSvc); return })

	initMetric("installHResult", func() (e error) { installHResult, e = metrics.NewString(cablib.MetricRoot+"installHResult", cablib.MetricSvc); return })
	initMetric("searchHResult", func() (e error) { searchHResult, e = metrics.NewString(cablib.MetricRoot+"searchHResult", cablib.MetricSvc); return })

	return firstErr
}

func setRebootMetric() {
	rbr, err := cablib.RebootRequired()
	if err != nil {
		deck.ErrorA(err).With(eventID(cablib.EvtErrMetricReport)).Go()
		return
	}

	if rebootRequired != nil {
		if err := rebootRequired.Set(rbr); err != nil {
			deck.ErrorA(err).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}

	if rbr {
		rebootEvent <- rbr
	}
}

func enforce() error {
	ctx := context.Background()
	updates, err := enforcement.Get()
	if err != nil {
		return fmt.Errorf("error retrieving required updates: %v", err)
	}
	if enforcedUpdateCount != nil {
		if err := enforcedUpdateCount.Set(int64(len(updates.Required))); err != nil {
			deck.ErrorfA("Error posting metric:\n%v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}
	var failures error
	if len(updates.Required) > 0 {
		i := installCmd{kbs: strings.Join(updates.Required, ",")}
		if err := i.installUpdates(ctx); err != nil {
			failures = fmt.Errorf("error enforcing required updates: %v", err)
			deck.ErrorA(failures).With(eventID(cablib.EvtErrInstallFailure)).Go()
		}
	}
	if len(updates.Hidden) > 0 {
		if err := hide(NewKBSetFromSlice(updates.Hidden)); err != nil {
			failures = fmt.Errorf("error hiding updates: %v", err)
			deck.ErrorA(failures).With(eventID(cablib.EvtErrHide)).Go()
		}
	}
	if len(updates.HiddenUpdateID) > 0 {
		if err := hideByUpdateID(updates.HiddenUpdateID); err != nil {
			failures = fmt.Errorf("error hiding updates by update ID: %v", err)
			deck.ErrorA(failures).With(eventID(cablib.EvtErrHide)).Go()
		}
	}
	return failures
}

func initDriverExclusion() error {
	updates, err := enforcement.Get()
	if err != nil {
		return fmt.Errorf("error retrieving required updates: %v", err)
	}
	excludedDrivers.set(updates.ExcludedDrivers)
	return nil
}

func runScheduledInstall(ctx context.Context) {
	i := installCmd{Interactive: false}
	err := i.installUpdates(ctx)
	if err != nil {
		deck.ErrorfA("Error installing system updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
	}
	if updateInstallSuccess != nil {
		if e := updateInstallSuccess.Set(err == nil); e != nil {
			deck.ErrorfA("Error posting metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}
	setRebootMetric()
}

func handleDefaultTimer(ctx context.Context) {
	runScheduledInstall(ctx)
}


func isWindowOpen(maintOpenDay, maintCloseDay int, ahOpens, ahCloses, now time.Time) bool {
	trimmedOpen := ahOpens.Add(time.Hour)
	trimmedClose := ahCloses.Add(-time.Hour)
	today := now.Day()
	deck.InfofA("Active Hours schedule found:\nNow: %v\nTrimmed Active Hours Open Time: %v\nTrimmed Active Hours Close Time: %v\nToday: %v\nMaintenance Open Day: %v\nMaintenance Close Day: %v\n", now, trimmedOpen, trimmedClose, today, maintOpenDay, maintCloseDay).With(eventID(cablib.EvtMisc)).Go()
	return trimmedOpen.Before(now) && trimmedClose.After(now) && (today >= maintOpenDay && today <= maintCloseDay)
}

func handleAukeraTimer(ctx context.Context) {
	s, err := client.Label(int(config.AukeraPort), config.AukeraName)
	if err != nil {
		deck.ErrorfA("Error getting maintenance window %q with error:\n%v", config.AukeraName, err).With(eventID(cablib.EvtErrMaintWindow)).Go()
		return
	}
	if *runInDebug {
		fmt.Printf("Cabbie maintenance window schedule:\n%+v", s)
	}
	if len(s) == 0 {
		deck.ErrorfA("Aukera maintenance window label %q not found, skipping update check...", config.AukeraName).With(eventID(cablib.EvtErrMaintWindow)).Go()
		return
	}
	if config.ActiveHoursEnabled == 1 {
		deck.InfofA("Active Hours enabled: checking for active_hours schedule.").With(eventID(cablib.EvtMisc)).Go()
		ah, err := client.Label(int(config.AukeraPort), `active_hours`)
		if err != nil {
			deck.ErrorfA("Error getting maintenance window %q with error:\n%v", `active_hours`, err).With(eventID(cablib.EvtErrMaintWindow)).Go()
			return
		}
		if len(ah) == 0 {
			deck.ErrorfA("Aukera maintenance window label %q not found, skipping update check...", `active_hours`).With(eventID(cablib.EvtErrMaintWindow)).Go()
			return
		}
		if isWindowOpen(s[0].Opens.Day(), s[0].Closes.Day(), ah[0].Opens, ah[0].Closes, time.Now()) {
			deck.InfofA("Active Hours + Maintenance window open: Starting installation process.").With(eventID(cablib.EvtInstall)).Go()
			runScheduledInstall(ctx)
		}
	} else {
		deck.InfofA("Active Hours disabled: using standard maintenance window schedule.").With(eventID(cablib.EvtMisc)).Go()
		if s[0].State == "open" {
			deck.InfofA("Maintenance window open: Starting installation process.").With(eventID(cablib.EvtInstall)).Go()
			runScheduledInstall(ctx)
		}
	}
}

func handleListTimer(ctx context.Context) {
	requiredUpdates, optionalUpdates, err := listUpdates(false, false)
	if listUpdateSuccess != nil {
		if e := listUpdateSuccess.Set(err == nil); e != nil {
			deck.ErrorfA("Error posting listUpdateSuccess metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}
	if err != nil {
		deck.ErrorfA("Error getting the list of updates:\n%v", err).With(eventID(cablib.EvtErrQueryFailure)).Go()
		return
	}
	if requiredUpdateCount != nil {
		if err := requiredUpdateCount.Set(int64(len(requiredUpdates))); err != nil {
			deck.ErrorfA("Error posting requiredUpdateCount metric:\n%v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}

	if len(requiredUpdates) == 0 {
		deck.InfoA("No required updates needed to install.").With(eventID(cablib.EvtNoUpdates)).Go()
		return
	}

	deck.InfofA("Found %d required updates.\nRequired updates:\n%s\nOptional updates:\n%s",
		len(requiredUpdates),
		strings.Join(requiredUpdates, "\n\n"),
		strings.Join(optionalUpdates, "\n\n"),
	).With(eventID(cablib.EvtUpdatesFound)).Go()

	if config.EnableNotifications == 1 {
		if err := notification.NewAvailableUpdateMessage().Push(ctx); err != nil {
			deck.ErrorfA("Failed to create notification:\n%v", err).With(eventID(cablib.EvtErrNotifications)).Go()
		}
	}

	if config.Deadline != 0 {
		i := installCmd{Interactive: false, deadlineOnly: true}
		if err := i.installUpdates(ctx); err != nil {
			deck.ErrorfA("Error installing system updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
		}
	}
}

func handleVirusTimer(ctx context.Context) {
	i := installCmd{Interactive: false, virusDef: true}
	err := i.installUpdates(ctx)
	if virusUpdateSuccess != nil {
		if e := virusUpdateSuccess.Set(err == nil); e != nil {
			deck.ErrorfA("Error posting virusUpdateSuccess metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}
	if err != nil {
		deck.ErrorfA("Error installing virus definitions:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
	}
}

func handleDriverTimer(ctx context.Context) {
	i := installCmd{Interactive: false, drivers: true}
	err := i.installUpdates(ctx)
	if driverUpdateSuccess != nil {
		if e := driverUpdateSuccess.Set(err == nil); e != nil {
			deck.ErrorfA("Error posting driverUpdateSuccess metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
		}
	}
	if err != nil {
		deck.ErrorfA("Error installing drivers:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
	}
	setRebootMetric()
}

func handleEnforcementTimer() {
	if err := enforce(); err != nil {
		deck.ErrorfA("Error enforcing one or more updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
	}
}

func handleEnforcementTrigger(file string) {
	deck.InfofA("Enforcement triggered by change in file %q.", file).With(eventID(cablib.EvtEnforcementChange)).Go()
	handleEnforcementTimer()
}

func handleRebootTrigger() {
	go func() {
		if rebootActive.CompareAndSwap(false, true) {
			defer rebootActive.Store(false)
			deck.InfoA("Reboot initiated...").With(eventID(cablib.EvtReboot)).Go()
			t, err := cablib.RebootTime()
			if err != nil {
				deck.ErrorfA("Error getting reboot time: %v", err).With(eventID(cablib.EvtErrPowerMgmt)).Go()
				return
			}
			if t.IsZero() {
				deck.InfoA("Zero time returned, no reboot defined.").With(eventID(cablib.EvtMisc)).Go()
				return
			}
			deck.InfofA("Reboot time is %s", t.String()).With(eventID(cablib.EvtMisc)).Go()
			if err := cablib.SystemReboot(context.Background(), t); err != nil {
				deck.ErrorfA("SystemReboot() error:\n%v", err).With(eventID(cablib.EvtErrPowerMgmt)).Go()
			}
		}
	}()
}

func runMainLoop(ctx context.Context) error {
	if err := notification.CleanNotifications(cablib.SvcName); err != nil {
		deck.ErrorfA("Error clearing old notifications:\n%v", err).With(eventID(cablib.EvtErrNotifications)).Go()
	}

	if config.EnableThirdParty == 1 {
		if err := enableThirdPartyUpdates(); err != nil {
			deck.ErrorfA("Error configuring third party updates:\n%v", err).With(eventID(cablib.EvtErrMisc)).Go()
		}
	}

	setRebootMetric()

	// Initialize service tickers.
	t := initTickers()
	defer t.stop()

	// If a profiling port is specified, start an HTTP server with graceful shutdown on context cancellation.
	if config.PprofPort != 0 {
		srv := &http.Server{Addr: fmt.Sprintf("localhost:%d", config.PprofPort)}
		go func() {
			<-ctx.Done()
			srv.Shutdown(context.Background())
		}()
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				deck.ErrorfA("pprof server error: %v", err).With(eventID(cablib.EvtErrMisc)).Go()
			}
		}()
	}

	// Run filesystem watcher for required updates configuration.
	enforcedFile := make(chan string)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			err := enforcement.Watcher(ctx, enforcedFile)
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				deck.ErrorfA("failed to initialize enforcement config watcher; relying on default enforcement schedule: %v", err).With(eventID(cablib.EvtErrEnforcement)).Go()
			}
			if enforcementWatcherFailures != nil {
				if err := enforcementWatcherFailures.Increment(); err != nil {
					deck.ErrorfA("unable to increment enforcementWatcherFailures metric: %v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Minute):
			}
		}
	}()

	if config.AukeraEnabled == 1 {
		deck.InfoA("Host configured to use Aukera. Ignoring default timer.").With(eventID(cablib.EvtMisc)).Go()
		t.Default.Stop()
	} else {
		deck.InfoA("Using default update interval.").With(eventID(cablib.EvtMisc)).Go()
		t.Aukera.Stop()
	}

	if config.InstallVirusDefs == 0 {
		t.Virus.Stop()
	}

	if config.InstallDrivers == 0 {
		t.Driver.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			deck.InfoA("Main loop stopping due to context cancellation.").With(eventID(cablib.EvtMisc)).Go()
			return ctx.Err()
		case <-t.Default.C:
			handleDefaultTimer(ctx)
		case <-t.Aukera.C:
			handleAukeraTimer(ctx)
		case <-t.List.C:
			handleListTimer(ctx)
		case <-t.Virus.C:
			handleVirusTimer(ctx)
		case <-t.Driver.C:
			handleDriverTimer(ctx)
		case file := <-enforcedFile:
			handleEnforcementTrigger(file)
		case <-t.Enforcement.C:
			handleEnforcementTimer()
		case <-rebootEvent:
			handleRebootTrigger()
		}
	}
}

// Execute starts the internal goroutine and waits for service signals from Windows.
func (m winSvc) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	errch := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes <- svc.Status{State: svc.StartPending}
	go func() {
		errch <- runMainLoop(ctx)
	}()
	deck.InfoA("Service started.").With(eventID(cablib.EvtServiceStarted)).Go()
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		// Watch for the cabbie goroutine to fail for some reason.
		case err := <-errch:
			if err != nil && err != context.Canceled {
				deck.ErrorfA("Cabbie goroutine has failed: %v", err).With(eventID(cablib.EvtErrService)).Go()
			}
			break loop
		// Watch for service signals.
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				cancel()
				break loop
			default:
				deck.ErrorfA("Unexpected control request #%d", c).With(eventID(cablib.EvtErrService)).Go()
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return ssec, errno
}

func enableThirdPartyUpdates() error {
	m, err := servicemgr.InitMgrService()
	if err != nil {
		return fmt.Errorf("failed to initialize Windows update service manager: %v", err)
	}
	defer m.Close()

	r, err := m.QueryServiceRegistration(servicemgr.MicrosoftUpdate)
	if err != nil {
		return fmt.Errorf("failed to query third party service registration status: %v", err)
	}

	if r {
		return nil
	}

	return m.AddService(servicemgr.MicrosoftUpdate)
}

// Lower the priority of the Cabbie process to IDLE_PRIORITY_CLASS in the OS scheduler and begin
// background processing to limit user impact see:
// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-setpriorityclass
func lowerProcessPriority() error {
	return windows.SetPriorityClass(windows.CurrentProcess(), windows.IDLE_PRIORITY_CLASS)
}

func main() {
	flag.Parse()
	var err error

	if !*runNormalPriority {
		if err := lowerProcessPriority(); err != nil {
			deck.ErrorfA("Failed to lower process priority: %v", err).With(eventID(cablib.EvtErrMisc)).Go()
		}
	}

	if *runInDebug {
		deck.Add(logger.Init(os.Stdout, 0))
	} else {
		if *verbose {
			deck.Add(logger.Init(os.Stdout, 0))
		}
		evt, err := eventlog.Init(cablib.LogSrcName)
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
		deck.Add(evt)
	}
	defer deck.Close()

	// Load Cabbie config settings.
	config = newSettings()
	if err = config.regLoad(cablib.RegPath()); err != nil {
		deck.ErrorfA("Failed to load Cabbie config, using defaults:\n%v\nError:%v", config, err).With(eventID(cablib.EvtErrConfig)).Go()
	}

	// If a profiling port is specified, start an HTTP server
	if config.PprofPort != 0 {
		go func() {
			http.ListenAndServe(fmt.Sprintf("localhost:%d", config.PprofPort), nil)
		}()
	}

	isSvc, err := svc.IsWindowsService()
	if err != nil {
		deck.ErrorfA("Failed to determine if we are running in an interactive session: %v", err).With(eventID(cablib.EvtErrMisc)).Go()
		os.Exit(2)
	}

	// Initialize metrics.
	if err := initMetrics(); err != nil {
		deck.ErrorA(err).With(eventID(cablib.EvtErrMetricReport)).Go()
	}

	comshim.Add(1)
	defer comshim.Done()

	// Running as Service.
	if isSvc && len(os.Args) == 1 {
		if err := startService(*runInDebug); err != nil {
			deck.ErrorfA("Failed to run service: %v", err).With(eventID(cablib.EvtErrService)).Go()
			os.Exit(2)
		}
		os.Exit(0)
	}

	// Running Interactively.
	ctx := context.Background()

	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	subcommands.Register(&hideCmd{}, "Update management")
	subcommands.Register(&historyCmd{}, "Update management")
	subcommands.Register(&installCmd{Interactive: true}, "Update management")
	subcommands.Register(&listCmd{}, "Update management")
	subcommands.Register(&rebootCmd{}, "Reboot management")
	subcommands.Register(&serviceCmd{}, "Service registration management")
	subcommands.Register(&wsusCmd{}, "WSUS management")

	if *runInDebug {
		if err := startService(true); err != nil {
			deck.ErrorfA("Failed to run service in debug mode: %v", err).With(eventID(cablib.EvtErrService)).Go()
			os.Exit(2)
		}
	}

	os.Exit(int(subcommands.Execute(ctx)))
}
