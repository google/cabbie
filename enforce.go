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

// +build windows

// TODO: Add support for hidden KBs

// Enforce implements filesystem watching for configured required updates.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/cabbie/cablib"
	"github.com/google/logger"
	"gopkg.in/fsnotify/fsnotify.v1"
)

const enforceDir = "C:\\ProgramData\\Cabbie"

type enforcement struct {
	Required []string `json:"required"`
}

func enforcements(path string) (enforcement, error) {
	path = filepath.Clean(path)
	// Enforcement files must live in Cabbie's directory. If given a relative path,
	// attempt a lookup by joining Cabbie's path and the given string.
	if !filepath.IsAbs(path) {
		path = filepath.Join(enforceDir, path)
	}
	if filepath.Ext(path) != ".json" {
		return enforcement{}, fmt.Errorf("getEnforcement: file %q is not json", path)
	}
	b, err := cablib.PathExists(path)
	if err != nil {
		return enforcement{}, fmt.Errorf("getEnforcement: error determining %q existence: %v", path, err)
	}
	if !b {
		return enforcement{}, fmt.Errorf("getEnforcement: %q doesn't exist", path)
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return enforcement{}, fmt.Errorf("getEnforcement: error reading file %q: %v", path, err)
	}
	var e enforcement
	if err := json.Unmarshal(data, &e); err != nil {
		return enforcement{}, fmt.Errorf("getEnforcement: error unmarshalling enforcement %q: %v", path, err)
	}
	return e, nil
}

func allEnforcements() (enforcement, error) {
	files, err := ioutil.ReadDir(enforceDir)
	if err != nil {
		return enforcement{}, err
	}
	var e enforcement
	for _, f := range files {
		p := filepath.Join(enforceDir, f.Name())
		kbs, err := enforcements(p)
		if err != nil {
			logger.Error(fmt.Sprintf("getAllEnforcements: error getting updates from %q:\n%v", p, err))
			continue
		}
		e.Required = append(e.Required, kbs.Required...)
	}
	if err := enforcedUpdateCount.Set(int64(len(e.Required))); err != nil {
		logger.Error(fmt.Sprintf("Error posting metric:\n%v", err))
	}
	e.dedupe()
	return e, nil
}

func (e *enforcement) dedupe() {
	u := make([]string, 0)
	m := make(map[string]bool)
	for _, v := range e.Required {
		if !m[v] {
			m[v] = true
			u = append(u, v)
		}
	}

	e.Required = u
}

func (e *enforcement) install() error {
	if len(e.Required) == 0 {
		logger.Error("No enforced updates defined.")
		return nil
	}
	i := installCmd{kbs: strings.Join(e.Required, ",")}
	return i.installUpdates()
}

// Filesystem watcher for required updates. This is meant to install required updates as soon as they are configured.
// All configured required updates are read on a schedule (see cabbie.go t.Enforcement ticker usage) to ensure required
// updates are installed even if a filesystem event is missed.
func runEnforcementWatcher(file chan<- string) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("enforce: error creating filesystem watcher:\n%v", err)
	}
	defer fsw.Close()

	exist, err := cablib.PathExists(enforceDir)
	if err != nil {
		return fmt.Errorf("enforce: error checking existence of %q:\n%v", enforceDir, err)
	}
	if !exist {
		if err := os.MkdirAll(enforceDir, 0664); err != nil {
			return fmt.Errorf("enforce: error creating %q:\n%v", enforceDir, err)
		}
	}

	if err := fsw.Add(enforceDir); err != nil {
		return fmt.Errorf("enforce: error adding %q to filesystem watcher:\n%v", enforceDir, err)
	}

	for {
		evt := <-fsw.Events
		if cablib.SliceContains([]fsnotify.Op{fsnotify.Write, fsnotify.Create}, evt.Op) {
			file <- evt.Name
		}
	}
}
