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
	"time"

	"flag"
	"github.com/google/cabbie/metrics"
	"github.com/google/cabbie/notification"
	"github.com/google/cabbie/cablib"
	"github.com/google/cabbie/enforcement"
	"github.com/google/cabbie/servicemgr"
	"github.com/google/aukera/client"
	"github.com/scjalliance/comshim"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc"
	"github.com/google/subcommands"
)

var (
	elog             debug.Log
	runInDebug       = flag.Bool("debug", false, "Run in debug mode")
	config           = new(Settings)
	categoryDefaults = []string{"Critical Updates", "Definition Updates", "Security Updates"}
	rebootEvent      = make(chan bool, 10)
	rebootActive     = false

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
		elog.Info(cablib.EvtErrConfig,
			fmt.Sprintf("AukeraName not found in registry, using default Name:\n%v", s.AukeraName))
	}

	if m, _, err := k.GetStringsValue("RequiredCategories"); err == nil {
		s.RequiredCategories = m
	} else {
		elog.Info(cablib.EvtErrConfig,
			fmt.Sprintf("RequiredCategories not found in registry, using default categories:\n%v", s.RequiredCategories))
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
	elog.Info(cablib.EvtServiceStarting, fmt.Sprintf("Starting %s service.", cablib.SvcName))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	if err := run(cablib.SvcName, winSvc{}); err != nil {
		return fmt.Errorf("%s service failed. %v", cablib.SvcName, err)
	}
	elog.Info(cablib.EvtServiceStopped, fmt.Sprintf("%s service stopped.", cablib.SvcName))
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
		elog.Error(cablib.EvtErrMetricReport, err.Error())
		return
	}

	if err := rebootRequired.Set(rbr); err != nil {
		elog.Error(cablib.EvtErrMetricReport, err.Error())
	}

	if rbr {
		rebootEvent <- rbr
	}
}

func enforce() error {
	kbs, err := enforcement.Get()
	if err != nil {
		return fmt.Errorf("error retrieving required updates: %v", err)
	}
	if err := enforcedUpdateCount.Set(int64(len(kbs.Required))); err != nil {
		elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting metric:\n%v", err))
	}
	var failures error
	if len(kbs.Required) > 0 {
		i := installCmd{kbs: strings.Join(kbs.Required, ",")}
		if err := i.installUpdates(); err != nil {
			failures = fmt.Errorf("error enforcing required updates: %v", err)
			elog.Error(cablib.EvtErrInstallFailure, failures.Error())
		}
	}
	if len(kbs.Hidden) > 0 {
		if err := hide(NewKBSetFromSlice(kbs.Hidden)); err != nil {
			failures = fmt.Errorf("error hiding updates: %v", err)
			elog.Error(cablib.EvtErrHide, failures.Error())
		}
	}
	return failures
}

func runMainLoop() error {
	if err := notification.CleanNotifications(cablib.SvcName); err != nil {
		elog.Error(cablib.EvtErrNotifications, fmt.Sprintf("Error clearing old notifications:\n%v", err))
	}

	if config.EnableThirdParty == 1 {
		if err := enableThirdPartyUpdates(); err != nil {
			elog.Error(cablib.EvtErrMisc, fmt.Sprintf("Error configuring third party updates:\n%v", err))
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
			err := enforcement.Watcher(enforcedFile)
			elog.Error(cablib.EvtErrEnforcement, fmt.Sprintf("failed to initialize enforcement config watcher; relying on enforcement schedule: %v", err))
			if err := enforcementWatcherFailures.Increment(); err != nil {
				elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("unable to increment enforcementWatcherFailures metric: %v", err))
			}
			time.Sleep(15 * time.Minute)
		}
	}()

	if config.AukeraEnabled == 1 {
		elog.Info(cablib.EvtMisc, "Host configured to use Aukera. Ignoring default timer.")
		t.Default.Stop()
	} else {
		elog.Info(cablib.EvtMisc, "Using default update interval.")
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
				elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting metric:\n%v", e))
			}
			setRebootMetric()
			if err != nil {
				elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Error installing system updates:\n%v", err))
			}
		case <-t.Aukera.C:
			s, err := client.Label(int(config.AukeraPort), config.AukeraName)
			if err != nil {
				elog.Error(cablib.EvtErrMaintWindow, fmt.Sprintf("Error getting maintenance window %q with error:\n%v", config.AukeraName, err))
				break
			}
			if *runInDebug {
				fmt.Printf("Cabbie maintenance window schedule:\n%+v", s)
			}
			if len(s) == 0 {
				elog.Error(cablib.EvtErrMaintWindow,
					fmt.Sprintf("Aukera maintenance window label %q not found, skipping update check...", config.AukeraName))
				break
			}
			if s[0].State == "open" {
				i := installCmd{Interactive: false}
				err := i.installUpdates()
				if e := updateInstallSuccess.Set(err == nil); e != nil {
					elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting updateInstallSuccess metric:\n%v", e))
				}
				setRebootMetric()
				if err != nil {
					elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Error installing system updates:\n%v", err))
				}
			}
		case <-t.List.C:
			setRebootMetric()
			requiredUpdates, optionalUpdates, err := listUpdates(true)
			if e := listUpdateSuccess.Set(err == nil); e != nil {
				elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting listUpdateSuccess metric:\n%v", e))
			}
			if err != nil {
				elog.Error(cablib.EvtErrQueryFailure, fmt.Sprintf("Error getting the list of updates:\n%v", err))
				break
			}
			if err := requiredUpdateCount.Set(int64(len(requiredUpdates))); err != nil {
				elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting requiredUpdateCount metric:\n%v", err))
			}

			if len(requiredUpdates) == 0 {
				elog.Info(cablib.EvtNoUpdates, "No required updates needed to install.")
				break
			}

			elog.Info(cablib.EvtUpdatesFound, fmt.Sprintf("Found %d required updates.\nRequired updates:\n%s\nOptional updates:\n%s",
				len(requiredUpdates),
				strings.Join(requiredUpdates, "\n\n"),
				strings.Join(optionalUpdates, "\n\n")),
			)

			if config.NotifyAvailable == 1 {
				if err := notification.NewAvailableUpdateMessage().Push(); err != nil {
					elog.Error(cablib.EvtErrNotifications, fmt.Sprintf("Failed to create notification:\n%v", err))
				}
			}

			if config.Deadline != 0 {
				i := installCmd{Interactive: false, deadlineOnly: true}
				if err := i.installUpdates(); err != nil {
					elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Error installing system updates:\n%v", err))
				}
			}
		case <-t.Virus.C:
			i := installCmd{Interactive: false, virusDef: true}
			err := i.installUpdates()
			if e := virusUpdateSuccess.Set(err == nil); e != nil {
				elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting virusUpdateSuccess metric:\n%v", err))
			}
			if err != nil {
				elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Error installing virus definitions:\n%v", err))
				break
			}
		case <-t.Driver.C:
			i := installCmd{Interactive: false, drivers: true}
			err := i.installUpdates()
			if e := driverUpdateSuccess.Set(err == nil); e != nil {
				elog.Error(cablib.EvtErrMetricReport, fmt.Sprintf("Error posting driverUpdateSuccess metric:\n%v", e))
			}
			if err != nil {
				elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Error installing drivers:\n%v", err))
			}
			setRebootMetric()
		case file := <-enforcedFile:
			elog.Info(cablib.EvtEnforcementChange, fmt.Sprintf("Enforcement triggered by change in file %q.", file))
			if err := enforce(); err != nil {
				elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Error enforcing one or more updates:\n%v", err))
			}
		case <-t.Enforcement.C:
			if err := enforce(); err != nil {
				elog.Error(cablib.EvtErrInstallFailure, fmt.Sprintf("Error enforcing one or more updates:\n%v", err))
			}
		case <-rebootEvent:
			go func() {
				if !(rebootActive) {
					rebootActive = true
					elog.Info(cablib.EvtReboot, "Reboot initiated...")
					t, err := cablib.RebootTime()
					if err != nil {
						elog.Error(cablib.EvtErrPowerMgmt, fmt.Sprintf("Error getting reboot time: %v", err))
						return
					}
					if t.IsZero() {
						elog.Info(cablib.EvtMisc, "Zero time returned, no reboot defined.")
						return
					}
					if err := cablib.SystemReboot(t); err != nil {
						elog.Error(cablib.EvtErrPowerMgmt, fmt.Sprintf("SystemReboot() error:\n%v", err))
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
	elog.Info(cablib.EvtServiceStarted, "Service started.")
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		// Watch for the cabbie goroutine to fail for some reason.
		case err := <-errch:
			elog.Error(cablib.EvtErrService, fmt.Sprintf("Cabbie goroutine has failed: %v", err))
			break loop
		// Watch for service signals.
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			default:
				elog.Error(cablib.EvtErrService, fmt.Sprintf("Unexpected control request #%d", c))
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
		elog = debug.New(cablib.LogSrcName)
	} else {
		elog, err = eventlog.Open(cablib.LogSrcName)
		if err != nil {
			fmt.Printf("Failed to create event: %v", err)
			os.Exit(2)
		}
	}
	defer elog.Close()

	// Load Cabbie config settings.
	config = newSettings()
	if err = config.regLoad(cablib.RegPath); err != nil {
		elog.Error(cablib.EvtErrConfig, fmt.Sprintf("Failed to load Cabbie config, using defaults:\n%v\nError:%v", config, err))
	}

	// If a profiling port is specified, start an HTTP server
	if config.PprofPort != 0 {
		go func() {
			http.ListenAndServe(fmt.Sprintf("localhost:%d", config.PprofPort), nil)
		}()
	}

	isSvc, err := svc.IsWindowsService()
	if err != nil {
		elog.Error(cablib.EvtErrMisc, fmt.Sprintf("Failed to determine if we are running in an interactive session: %v", err))
		os.Exit(2)
	}

	// Initialize metrics.
	if err := initMetrics(); err != nil {
		elog.Error(cablib.EvtErrMetricReport, err.Error())
	}

	comshim.Add(1)
	defer comshim.Done()

	// Running as Service.
	// TODO(b/147692789): move service logic into its own subcommand.
	if isSvc && len(os.Args) == 1 {
		if err := startService(*runInDebug); err != nil {
			elog.Error(cablib.EvtErrService, fmt.Sprintf("Failed to run service: %v", err))
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
			elog.Error(cablib.EvtErrService, fmt.Sprintf("Failed to run service in debug mode: %v", err))
			os.Exit(2)
		}
	}

	os.Exit(int(subcommands.Execute(ctx)))
}
