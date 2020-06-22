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

// Package metrics provides a library to post status metrics.
package metrics

import "sync"

type metricData struct {
	name    string
	service string
}

// Bool implements a Bool-type metric.
type Bool struct {
	value bool
	mu    sync.Mutex
	data  *metricData
}

// NewBool sets the metric to a new Bool value.
func NewBool(name, service string) (*Bool, error) {
	return &Bool{
		data: &metricData{
			name:    name,
			service: service,
		},
	}, nil
}

// Set sets the metric to a new bool value.
func (b *Bool) Set(value bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.value = value
	return nil
}

// Int implements a Int-type metric.
type Int struct {
	value int64
	mu    sync.Mutex
	data  *metricData
}

// NewInt sets the metric to a new Int value.
func NewInt(name, service string) (*Int, error) {
	return &Int{
		data: &metricData{
			name:    name,
			service: service,
		},
	}, nil
}

// Set sets the metric to a new int value.
func (i *Int) Set(value int64) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.value = value
	return nil
}

// NewCounter sets the metric to a new Int value.
func NewCounter(name, service string) (*Int, error) {
	return &Int{
		data: &metricData{
			name:    name,
			service: service,
		},
	}, nil
}

// Increment adds to the current int metric value.
func (i *Int) Increment() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.value++
	return nil
}

// String implements a String-type metric.
type String struct {
	value string
	mu    sync.Mutex
	data  *metricData
}

// NewString sets the metric to a new string value.
func NewString(name, service string) (*String, error) {
	return &String{
		data: &metricData{
			name:    name,
			service: service,
		},
	}, nil
}

// Set sets the metric to a new string value.
func (s *String) Set(value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.value = value
	return nil
}
