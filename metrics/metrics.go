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

// MetricData stores metric information.
type MetricData struct {
	Name    string
	service string
	mu      sync.Mutex
	Fields  map[string]interface{}
}

// AddBoolField adds a bool field to a metric.
func (m *MetricData) AddBoolField(name string, value bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Fields == nil {
		m.Fields = make(map[string]interface{})
	}
	m.Fields[name] = value
}

// AddStringField adds a string field to a metric.
func (m *MetricData) AddStringField(name, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Fields == nil {
		m.Fields = make(map[string]interface{})
	}
	m.Fields[name] = value
}

// Bool implements a Bool-type metric.
type Bool struct {
	Value bool
	mu    sync.Mutex
	Data  *MetricData
}

// NewBool sets the metric to a new Bool value.
func NewBool(name, service string) (*Bool, error) {
	return &Bool{
		Data: &MetricData{
			Name:    name,
			service: service,
		},
	}, nil
}

// Set sets the metric to a new bool value.
func (b *Bool) Set(value bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Value = value
	return nil
}

// Int implements a Int-type metric.
type Int struct {
	Value int64
	mu    sync.Mutex
	Data  *MetricData
}

// NewInt sets the metric to a new Int value.
func NewInt(name, service string) (*Int, error) {
	return &Int{
		Data: &MetricData{
			Name:    name,
			service: service,
		},
	}, nil
}

// Set sets the metric to a new int value.
func (i *Int) Set(value int64) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.Value = value
	return nil
}

// NewCounter sets the metric to a new Int value.
func NewCounter(name, service string) (*Int, error) {
	return &Int{
		Data: &MetricData{
			Name:    name,
			service: service,
		},
	}, nil
}

// Increment adds to the current int metric value.
func (i *Int) Increment() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.Value++
	return nil
}

// String implements a String-type metric.
type String struct {
	Value string
	mu    sync.Mutex
	Data  *MetricData
}

// NewString sets the metric to a new string value.
func NewString(name, service string) (*String, error) {
	return &String{
		Data: &MetricData{
			Name:    name,
			service: service,
		},
	}, nil
}

// Set sets the metric to a new string value.
func (s *String) Set(value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Value = value
	return nil
}
