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
	"golang.org/x/net/context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/cabbie/cablib"

	"gopkg.in/fsnotify.v1"
	"github.com/google/glazier/go/helpers"
)

var (
	errFileType    = errors.New("file is not json")
	errInvalidFile = errors.New("file path is invalid")
	errParsing     = errors.New("could not parse file content")

	enforceDirMu sync.RWMutex
	enforceDir   = filepath.Join(os.Getenv("ProgramData"), `\Cabbie`)
)

// SetEnforceDir sets the enforcement directory path.
func SetEnforceDir(path string) {
	enforceDirMu.Lock()
	defer enforceDirMu.Unlock()
	enforceDir = path
}

// EnforceDir returns the current enforcement directory path.
func EnforceDir() string {
	enforceDirMu.RLock()
	defer enforceDirMu.RUnlock()
	return enforceDir
}

// Enforcements track any externally configured update enforcements.
type Enforcements struct {
	Required        []string        `json:"required"`
	ExcludedDrivers []DriverExclude `json:"excluded-drivers"`
	Hidden          []string        `json:"hidden"`
	HiddenUpdateID  []string        `json:"hidden-UpdateID"`
}

// DriverExclude specifies criteria to exclude certain driver updates.
// A driver update is ignored by Cabbie if it matches all criteria.
type DriverExclude struct {
	DriverClass   string `json:"driver-class"`
	DriverDateVer string `json:"driver-date-version"`
}

func enforcements(path string) (Enforcements, error) {
	var e Enforcements
	path = filepath.Clean(path)
	if filepath.Ext(path) != ".json" {
		return e, fmt.Errorf("%w: %q", errFileType, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return e, fmt.Errorf("%w: %q", errInvalidFile, path)
		}
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
	dir := EnforceDir()
	files, err := os.ReadDir(dir)
	if err != nil {
		return ret, err
	}
	for _, f := range files {
		p := filepath.Join(dir, f.Name())
		e, err := enforcements(p)
		if err != nil {
			// TODO(mattl): surface errors here somehow
			continue
		}
		ret.Required = append(ret.Required, e.Required...)
		ret.Hidden = append(ret.Hidden, e.Hidden...)
		ret.ExcludedDrivers = append(ret.ExcludedDrivers, e.ExcludedDrivers...)
		ret.HiddenUpdateID = append(ret.HiddenUpdateID, e.HiddenUpdateID...)
	}
	ret.dedupe()
	return ret, nil
}

func unique[T comparable](list []T) []T {
	u := make([]T, 0, len(list))
	m := make(map[T]bool, len(list))
	for _, v := range list {
		if !m[v] {
			m[v] = true
			u = append(u, v)
		}
	}
	return u
}

func (e *Enforcements) dedupe() {
	e.Required = unique(e.Required)
	e.Hidden = unique(e.Hidden)
	e.HiddenUpdateID = unique(e.HiddenUpdateID)
	e.ExcludedDrivers = unique(e.ExcludedDrivers)
}

func setupWatcher(dir string) (*fsnotify.Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("enforce: error creating filesystem watcher:\n%v", err)
	}

	exist, err := helpers.PathExists(dir)
	if err != nil {
		fsw.Close()
		return nil, fmt.Errorf("enforce: error checking existence of %q:\n%v", dir, err)
	}
	if !exist {
		if err := os.MkdirAll(dir, 0664); err != nil {
			fsw.Close()
			return nil, fmt.Errorf("enforce: error creating %q:\n%v", dir, err)
		}
	}

	if err := fsw.Add(dir); err != nil {
		fsw.Close()
		return nil, fmt.Errorf("enforce: error adding %q to filesystem watcher:\n%v", dir, err)
	}

	return fsw, nil
}

func watchLoop(ctx context.Context, fsw *fsnotify.Watcher, file chan<- string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			if cablib.SliceContains([]fsnotify.Op{fsnotify.Write, fsnotify.Create}, evt.Op) {
				select {
				case file <- evt.Name:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			if err != nil {
				return fmt.Errorf("enforce: watcher error: %v", err)
			}
		}
	}
}

// Watcher runs a filesystem watcher for required updates. This is meant to install required updates as soon as they are configured.
// All configured required updates are read on a schedule (see cabbie.go t.Enforcement ticker usage) to ensure required
// updates are installed even if a filesystem event is missed.
func Watcher(ctx context.Context, file chan<- string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	dir := EnforceDir()
	fsw, err := setupWatcher(dir)
	if err != nil {
		return err
	}
	defer fsw.Close()

	return watchLoop(ctx, fsw, file)
}

