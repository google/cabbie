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

// Package enforcement implements filesystem watching for configured required updates.
package enforcement

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/cabbie/cablib"

	"gopkg.in/fsnotify.v1"
	"github.com/google/glazier/go/helpers"
)

var (
	errFileType    = errors.New("file is not json")
	errInvalidFile = errors.New("file path is invalid")
	errParsing     = errors.New("could not parse file content")

	enforceDir = filepath.Join(os.Getenv("ProgramData"), `\Cabbie`)
)

// Enforcements track any externally configured update enforcements.
type Enforcements struct {
	Required        []string        `json:"required"`
	ExcludedDrivers []DriverExclude `json:"excluded-drivers"`
	Hidden          []string        `json:"hidden"`
}

// DriverExclude specifies criteria to exclude certain driver updates.
// A driver update is ignored by Cabbie if it matches all criteria.
type DriverExclude struct {
	DriverClass string `json:"driver-class"`
	UpdateID    string `json:"update-id"`
}

func enforcements(path string) (Enforcements, error) {
	var e Enforcements
	path = filepath.Clean(path)
	if filepath.Ext(path) != ".json" {
		return e, fmt.Errorf("%w: %q", errFileType, path)
	}
	b, err := helpers.PathExists(path)
	if err != nil {
		return e, fmt.Errorf("error determining %q existence: %v", path, err)
	}
	if !b {
		return e, fmt.Errorf("%w: %q", errInvalidFile, path)
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return e, fmt.Errorf("error reading file %q: %v", path, err)
	}
	if err := json.Unmarshal(data, &e); err != nil {
		return e, fmt.Errorf("%w for %q: %v", errParsing, path, err)
	}
	return e, nil
}

// Get attempts to return all known external enforcements.
func Get() (Enforcements, error) {
	var ret Enforcements
	files, err := ioutil.ReadDir(enforceDir)
	if err != nil {
		return ret, err
	}
	for _, f := range files {
		p := filepath.Join(enforceDir, f.Name())
		e, err := enforcements(p)
		if err != nil {
			// TODO(mattl): surface errors here somehow
			continue
		}
		ret.Required = append(ret.Required, e.Required...)
		ret.Hidden = append(ret.Hidden, e.Hidden...)
		ret.ExcludedDrivers = append(ret.ExcludedDrivers, e.ExcludedDrivers...)
	}
	ret.dedupe()
	return ret, nil
}

// go generics are super new. The following two funcs should be merged
// into one generic one after the dust has settled.

func uniqueStrings(list []string) []string {
	u := make([]string, 0)
	m := make(map[string]bool)
	for _, v := range list {
		if !m[v] {
			m[v] = true
			u = append(u, v)
		}
	}
	return u
}

func uniqueDriverExclude(list []DriverExclude) []DriverExclude {
	u := make([]DriverExclude, 0)
	m := make(map[DriverExclude]bool)
	for _, v := range list {
		if !m[v] {
			m[v] = true
			u = append(u, v)
		}
	}
	return u
}

func (e *Enforcements) dedupe() {
	e.Required = uniqueStrings(e.Required)
	e.Hidden = uniqueStrings(e.Hidden)
	e.ExcludedDrivers = uniqueDriverExclude(e.ExcludedDrivers)
}

// Watcher runs a filesystem watcher for required updates. This is meant to install required updates as soon as they are configured.
// All configured required updates are read on a schedule (see cabbie.go t.Enforcement ticker usage) to ensure required
// updates are installed even if a filesystem event is missed.
func Watcher(file chan<- string) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("enforce: error creating filesystem watcher:\n%v", err)
	}
	defer fsw.Close()

	exist, err := helpers.PathExists(enforceDir)
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
