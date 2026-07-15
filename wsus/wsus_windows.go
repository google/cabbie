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

package wsus

import (
	"golang.org/x/net/context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/google/cabbie/cablib"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc/eventlog"
)

var (
	wlog   *eventlog.Log
	wlogMu sync.Mutex
)

func setLog(l *eventlog.Log) {
	wlogMu.Lock()
	defer wlogMu.Unlock()
	wlog = l
}

func logWarning(eid uint32, msg string) {
	wlogMu.Lock()
	l := wlog
	wlogMu.Unlock()
	if l != nil {
		l.Warning(eid, msg)
	}
}

func responseTime(ctx context.Context, name string) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s", name), nil)
	if err != nil {
		logWarning(3, fmt.Sprintf("Failed to create new http request: %v", err))
		return 0, err
	}

	start := time.Now()
	resp, err := http.DefaultTransport.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		logWarning(3, fmt.Sprintf("Failed to send GET request to: %v", err))
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		logWarning(3, fmt.Sprintf("Non-200 status code returned(%d)", resp.StatusCode))
		return 0, fmt.Errorf("non-200 status code: %d", resp.StatusCode)
	}

	return time.Since(start), nil
}

// Init will initialize the local update client with the desired WSUS config.
func Init(servers []string) (*WSUS, error) {
	var w WSUS
	var err error

	if len(servers) == 0 {
		w.ServerSelection = WindowsUpdate
		return &w, w.Clear()
	}

	el, err := eventlog.Open("Cabbie WSUS")
	if err != nil {
		return &w, err
	}
	setLog(el)

	ctx := context.Background()
	w.order(ctx, servers)

	if len(w.Servers) == 0 {
		w.ServerSelection = WindowsUpdate
		return &w, w.Clear()
	}

	if err := w.Set(0); err != nil {
		w.ServerSelection = WindowsUpdate
		return &w, fmt.Errorf("error setting WSUS config:\n%v", err)
	}
	w.ServerSelection = ManagedServer
	return &w, nil
}

type serverLatency struct {
	name    string
	latency time.Duration
}

// order returns a list of WSUS servers from fastest to slowest.
func (w *WSUS) order(ctx context.Context, servers []string) {
	w.Servers = nil
	var validServers []serverLatency
	for _, n := range servers {
		t, err := responseTime(ctx, n)
		if err != nil {
			logWarning(2, fmt.Sprintf("Skipping WSUS server %s as it appears to be unreachable: %v", n, err))
			continue
		}
		validServers = append(validServers, serverLatency{name: n, latency: t})
	}

	sort.Slice(validServers, func(i, j int) bool {
		return validServers[i].latency < validServers[j].latency
	})

	for _, s := range validServers {
		w.Servers = append(w.Servers, s.name)
	}
}

// Set configures the update client to use the requested WSUS server.
func (w *WSUS) Set(index int) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, cablib.WUReg, registry.ALL_ACCESS)
	if err != nil && err != registry.ErrNotExist {
		return err
	}
	if err == registry.ErrNotExist {
		k, _, err = registry.CreateKey(registry.LOCAL_MACHINE, cablib.WUReg, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
	}
	defer k.Close()

	if index < 0 || index > (len(w.Servers)-1) {
		return fmt.Errorf("requested index (%d) is out of selectable server range (%d)", index, (len(w.Servers) - 1))
	}
	name := w.Servers[index]
	url := fmt.Sprintf("https://%s", name)
	if err := k.SetStringValue("WUServer", url); err != nil {
		return err
	}
	w.CurrentServer = name

	if err := k.SetStringValue("WUStatusServer", url); err != nil {
		return err
	}

	sk, _, err := registry.CreateKey(k, "AU", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer sk.Close()

	return sk.SetDWordValue("UseWUServer", 1)
}

// Clear sets WSUS client configurations back to Windows defaults.
func (w *WSUS) Clear() error {
	w.CurrentServer = ""
	w.ServerSelection = WindowsUpdate

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, cablib.WUReg, registry.ALL_ACCESS)
	if err != nil && err != registry.ErrNotExist {
		return err
	}
	if err == registry.ErrNotExist {
		return nil
	}
	defer k.Close()

	err = k.DeleteValue("WUServer")
	if err != nil && err != registry.ErrNotExist {
		logWarning(4, fmt.Sprintf("Failed to delete WUServer registry value: %v", err))
	}
	err = k.DeleteValue("WUStatusServer")
	if err != nil && err != registry.ErrNotExist {
		logWarning(4, fmt.Sprintf("Failed to delete WUStatusServer registry value: %v", err))
	}

	auk, err := registry.OpenKey(k, "AU", registry.ALL_ACCESS)
	if err == registry.ErrNotExist {
		return nil
	} else if err != nil {
		return err
	}

	defer auk.Close()

	err = auk.DeleteValue("UseWUServer")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("Failed to delete UseWUServer registry value: %v", err)
	}
	return nil
}
