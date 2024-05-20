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
	"time"

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
	runInDebug       = flag.Bool("debug", false, "Run in debug mode")
	config           = new(Settings)
	categoryDefaults = []string{"Critical Updates", "Definition Updates", "Security Updates"}
	rebootEvent      = make(chan bool, 10)
	rebootActive     = false

	excludedDrivers driverExcludes

	// Metrics
	virusUpdateSuccess         = new(metrics.Bool)
	listUpdateSuccess          = new(metrics.Bool)
	driverUpdateSuccess        = new(metrics.Bool)
	updateInstallSuccess       = new(metrics.Bool)
	rebootRequired             = new(metrics.Bool)
	deviceIsPatched            = new(metrics.Bool)
	requiredUpdateCount        = new(metrics.Int)
	enforcedUpdateCount        = new(metrics.Int)
	enforcementWatcherFailures = new(metrics.Int)
	installHResult             = new(metrics.String)
	searchHResult              = new(metrics.String)

	eventID = eventlog.EventID
)

// Settings contains configurable options.
type Settings struct {
	WSUSServers, RequiredCategories                                                         []string
	UpdateDrivers, UpdateVirusDef, EnableThirdParty, RebootDelay, Deadline, NotifyAvailable uint64

	// Aukera Integration
	AukeraEnabled uint64
	AukeraPort    uint64
	AukeraName    string

	PprofPort uint64

	ScriptTimeout time.Duration
}

type tickers struct {
	Default, Aukera, List, Virus, Driver, Enforcement *time.Ticker
}

type driverExcludes struct {
	mutex sync.Mutex
	e     []enforcement.DriverExclude
}

func (d *driverExcludes) set(v []enforcement.DriverExclude) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.e = v
}

func (d *driverExcludes) get() []enforcement.DriverExclude {
	d.mutex.Lock()
	defer d.mutex.Unlock()
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
		AukeraName:         cablib.SvcName,
		RequiredCategories: categoryDefaults,
		UpdateVirusDef:     1,
		RebootDelay:        21600,
		Deadline:           14,
		NotifyAvailable:    1,
		AukeraPort:         9119,
		ScriptTimeout:      10 * time.Minute,
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
		deck.InfofA(
			"AukeraName not found in registry, using default Name:\n%v", s.AukeraName).With(eventID(cablib.EvtErrConfig)).Go()
	}

	if m, _, err := k.GetStringsValue("RequiredCategories"); err == nil {
		s.RequiredCategories = m
	} else {
		deck.InfofA("RequiredCategories not found in registry, using default categories:\n%v", s.RequiredCategories).With(eventID(cablib.EvtErrConfig)).Go()
	}

	if i, _, err := k.GetIntegerValue("EnableThirdParty"); err == nil {
		s.EnableThirdParty = i
	}
	if i, _, err := k.GetIntegerValue("UpdateDrivers"); err == nil {
		s.UpdateDrivers = i
	}
	if i, _, err := k.GetIntegerValue("UpdateVirusDef"); err == nil {
		s.UpdateVirusDef = i
	}
	if i, _, err := k.GetIntegerValue("RebootDelay"); err == nil {
		s.RebootDelay = i
	}
	if i, _, err := k.GetIntegerValue("Deadline"); err == nil {
		s.Deadline = i
	}
	if i, _, err := k.GetIntegerValue("NotifyAvailable"); err == nil {
		s.NotifyAvailable = i
	}
	if i, _, err := k.GetIntegerValue("AukeraEnabled"); err == nil {
		s.AukeraEnabled = i
	}
	if i, _, err := k.GetIntegerValue("AukeraPort"); err == nil {
		s.AukeraPort = i
	}
	if i, _, err := k.GetIntegerValue("PprofPort"); err == nil {
		s.PprofPort = i
	}
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
	var err error

	// bool metrics
	virusUpdateSuccess, err = metrics.NewBool(cablib.MetricRoot+"virusUpdateSuccess", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize virusUpdateSuccess metric: %v", err)
	}
	listUpdateSuccess, err = metrics.NewBool(cablib.MetricRoot+"listUpdateSuccess", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize listUpdateSuccess metric: %v", err)
	}
	driverUpdateSuccess, err = metrics.NewBool(cablib.MetricRoot+"driverUpdateSuccess", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize driverUpdateSuccess metric: %v", err)
	}
	updateInstallSuccess, err = metrics.NewBool(cablib.MetricRoot+"updateInstallSuccess", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize updateInstallSuccess metric: %v", err)
	}
	rebootRequired, err = metrics.NewBool(cablib.MetricRoot+"rebootRequired", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize rebootRequired metric: %v", err)
	}
	deviceIsPatched, err = metrics.NewBool(cablib.MetricRoot+"deviceIsPatched", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize deviceIsPatched metric: %v", err)
	}

	// integer metrics
	requiredUpdateCount, err = metrics.NewInt(cablib.MetricRoot+"requiredUpdateCount", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize requiredUpdateCount metric: %v", err)
	}
	enforcedUpdateCount, err = metrics.NewInt(cablib.MetricRoot+"enforcedUpdateCount", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize enforcedUpdateCount metric: %v", err)
	}
	enforcementWatcherFailures, err = metrics.NewCounter(cablib.MetricRoot+"enforcementWatcherFailures", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to create enforcementWatcherFailures metric: %v", err)
	}

	// string metrics
	installHResult, err = metrics.NewString(cablib.MetricRoot+"installHResult", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize installHResult metric: %v", err)
	}
	searchHResult, err = metrics.NewString(cablib.MetricRoot+"searchHResult", cablib.MetricSvc)
	if err != nil {
		return fmt.Errorf("unable to initialize searchHResult metric: %v", err)
	}

	return nil
}

func setRebootMetric() {
	rbr, err := cablib.RebootRequired()
	if err != nil {
		deck.ErrorA(err).With(eventID(cablib.EvtErrMetricReport)).Go()
		return
	}

	if err := rebootRequired.Set(rbr); err != nil {
		deck.ErrorA(err).With(eventID(cablib.EvtErrMetricReport)).Go()
	}

	if rbr {
		rebootEvent <- rbr
	}
}

func enforce() error {
	updates, err := enforcement.Get()
	if err != nil {
		return fmt.Errorf("error retrieving required updates: %v", err)
	}
	if err := enforcedUpdateCount.Set(int64(len(updates.Required))); err != nil {
		deck.ErrorfA("Error posting metric:\n%v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
	}
	var failures error
	if len(updates.Required) > 0 {
		i := installCmd{kbs: strings.Join(updates.Required, ",")}
		if err := i.installUpdates(); err != nil {
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

func runMainLoop() error {
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

	// Run filesystem watcher for required updates configuration.
	var enforcedFile = make(chan string)
	go func() {
		for {
			if err := enforcement.Watcher(enforcedFile); err == nil {
				deck.ErrorfA("failed to initialize enforcement config watcher; relying on default enforcement schedule: %v", err).With(eventID(cablib.EvtErrEnforcement)).Go()
			}
			if err := enforcementWatcherFailures.Increment(); err != nil {
				deck.ErrorfA("unable to increment enforcementWatcherFailures metric: %v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
			}
			time.Sleep(15 * time.Minute)
		}
	}()

	if config.AukeraEnabled == 1 {
		deck.InfoA("Host configured to use Aukera. Ignoring default timer.").With(eventID(cablib.EvtMisc)).Go()
		t.Default.Stop()
	} else {
		deck.InfoA("Using default update interval.").With(eventID(cablib.EvtMisc)).Go()
		t.Aukera.Stop()
	}

	if config.UpdateVirusDef == 0 {
		t.Virus.Stop()
	}

	if config.UpdateDrivers == 0 {
		t.Driver.Stop()
	}

	for {
		select {
		case <-t.Default.C:
			i := installCmd{Interactive: false}
			err := i.installUpdates()
			if e := updateInstallSuccess.Set(err == nil); e != nil {
				deck.ErrorfA("Error posting metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
			}
			setRebootMetric()
			if err != nil {
				deck.ErrorfA("Error installing system updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
			}
		case <-t.Aukera.C:
			s, err := client.Label(int(config.AukeraPort), config.AukeraName)
			if err != nil {
				deck.ErrorfA("Error getting maintenance window %q with error:\n%v", config.AukeraName, err).With(eventID(cablib.EvtErrMaintWindow)).Go()
				break
			}
			if *runInDebug {
				fmt.Printf("Cabbie maintenance window schedule:\n%+v", s)
			}
			if len(s) == 0 {
				deck.ErrorfA("Aukera maintenance window label %q not found, skipping update check...", config.AukeraName).With(eventID(cablib.EvtErrMaintWindow)).Go()
				break
			}
			if s[0].State == "open" {
				i := installCmd{Interactive: false}
				err := i.installUpdates()
				if e := updateInstallSuccess.Set(err == nil); e != nil {
					deck.ErrorfA("Error posting updateInstallSuccess metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
				}
				setRebootMetric()
				if err != nil {
					deck.ErrorfA("Error installing system updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
				}
			}
		case <-t.List.C:
			requiredUpdates, optionalUpdates, err := listUpdates(false, false)
			if e := listUpdateSuccess.Set(err == nil); e != nil {
				deck.ErrorfA("Error posting listUpdateSuccess metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
			}
			if err != nil {
				deck.ErrorfA("Error getting the list of updates:\n%v", err).With(eventID(cablib.EvtErrQueryFailure)).Go()
				break
			}
			if err := requiredUpdateCount.Set(int64(len(requiredUpdates))); err != nil {
				deck.ErrorfA("Error posting requiredUpdateCount metric:\n%v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
			}

			if len(requiredUpdates) == 0 {
				deck.InfoA("No required updates needed to install.").With(eventID(cablib.EvtNoUpdates)).Go()
				break
			}

			deck.InfofA("Found %d required updates.\nRequired updates:\n%s\nOptional updates:\n%s",
				len(requiredUpdates),
				strings.Join(requiredUpdates, "\n\n"),
				strings.Join(optionalUpdates, "\n\n"),
			).With(eventID(cablib.EvtUpdatesFound)).Go()

			if config.NotifyAvailable == 1 {
				if err := notification.NewAvailableUpdateMessage().Push(); err != nil {
					deck.ErrorfA("Failed to create notification:\n%v", err).With(eventID(cablib.EvtErrNotifications)).Go()
				}
			}

			if config.Deadline != 0 {
				i := installCmd{Interactive: false, deadlineOnly: true}
				if err := i.installUpdates(); err != nil {
					deck.ErrorfA("Error installing system updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
				}
			}
		case <-t.Virus.C:
			i := installCmd{Interactive: false, virusDef: true}
			err := i.installUpdates()
			if e := virusUpdateSuccess.Set(err == nil); e != nil {
				deck.ErrorfA("Error posting virusUpdateSuccess metric:\n%v", err).With(eventID(cablib.EvtErrMetricReport)).Go()
			}
			if err != nil {
				deck.ErrorfA("Error installing virus definitions:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
				break
			}
		case <-t.Driver.C:
			i := installCmd{Interactive: false, drivers: true}
			err := i.installUpdates()
			if e := driverUpdateSuccess.Set(err == nil); e != nil {
				deck.ErrorfA("Error posting driverUpdateSuccess metric:\n%v", e).With(eventID(cablib.EvtErrMetricReport)).Go()
			}
			if err != nil {
				deck.ErrorfA("Error installing drivers:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
			}
			setRebootMetric()
		case file := <-enforcedFile:
			deck.InfofA("Enforcement triggered by change in file %q.", file).With(eventID(cablib.EvtEnforcementChange)).Go()
			if err := enforce(); err != nil {
				deck.ErrorfA("Error enforcing one or more updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
			}
		case <-t.Enforcement.C:
			if err := enforce(); err != nil {
				deck.ErrorfA("Error enforcing one or more updates:\n%v", err).With(eventID(cablib.EvtErrInstallFailure)).Go()
			}
		case <-rebootEvent:
			go func() {
				if !(rebootActive) {
					rebootActive = true
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
					if err := cablib.SystemReboot(t); err != nil {
						deck.ErrorfA("SystemReboot() error:\n%v", err).With(eventID(cablib.EvtErrPowerMgmt)).Go()
					}
					rebootActive = false
				}
			}()
		}
	}
}

// Execute starts the internal goroutine and waits for service signals from Windows.
func (m winSvc) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {

	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	errch := make(chan error)

	changes <- svc.Status{State: svc.StartPending}
	go func() {
		errch <- runMainLoop()
	}()
	deck.InfoA("Service started.").With(eventID(cablib.EvtServiceStarted)).Go()
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		// Watch for the cabbie goroutine to fail for some reason.
		case err := <-errch:
			deck.ErrorfA("Cabbie goroutine has failed: %v", err).With(eventID(cablib.EvtErrService)).Go()
			break loop
		// Watch for service signals.
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
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

func main() {
	flag.Parse()
	var err error

	if *runInDebug {
		deck.Add(logger.Init(os.Stdout, 0))
	} else {
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
	if err = config.regLoad(cablib.RegPath); err != nil {
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
	subcommands.Register(&serviceCmd{}, "Service registration management")

	if *runInDebug {
		if err := startService(true); err != nil {
			deck.ErrorfA("Failed to run service in debug mode: %v", err).With(eventID(cablib.EvtErrService)).Go()
			os.Exit(2)
		}
	}

	os.Exit(int(subcommands.Execute(ctx)))
}
