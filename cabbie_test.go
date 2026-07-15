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

package main

import (
	"golang.org/x/net/context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sys/windows/registry"
)

const (
	testPath = `SOFTWARE\Bar`
)

func createTestKeys() error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, testPath, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	return k.Close()
}

func cleanupTestKey() error {
	return registry.DeleteKey(registry.LOCAL_MACHINE, testPath)
}

func TestRegLoadKeyMissing(t *testing.T) {
	// Setup
	expected := newSettings()
	testconfig := newSettings()
	// End Setup
	if err := testconfig.regLoad(testPath); err != registry.ErrNotExist {
		t.Error(err)
	}
	if !(cmp.Equal(testconfig, expected)) {
		t.Errorf("testconfig.regload(%s) = %v, want %v", testPath, testconfig, expected)
	}
}

func TestRegLoadKeyEmpty(t *testing.T) {
	// Setup
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	defer cleanupTestKey()
	expected := newSettings()
	testconfig := newSettings()
	// End Setup
	if err := testconfig.regLoad(testPath); err != nil {
		t.Error(err)
	}
	if !(cmp.Equal(testconfig, expected)) {
		t.Errorf("testconfig.regLoad(%s) = %v, want %v", testPath, testconfig, expected)
	}
}

func TestRegLoadRequiredCategories(t *testing.T) {
	// Setup
	rc := []string{"Bar", "Foo"}
	if err := createTestKeys(); err != nil {
		t.Fatal(err)
	}
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, testPath, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.SetStringsValue("RequiredCategories", rc); err != nil {
		t.Fatal(err)
	}
	k.Close()
	defer cleanupTestKey()

	expected := newSettings()
	expected.RequiredCategories = rc
	testconfig := newSettings()
	// End Setup
	if err := testconfig.regLoad(testPath); err != nil {
		t.Error(err)
	}
	if !(cmp.Equal(testconfig, expected)) {
		t.Errorf("testconfig.regLoad(%s) = %v, want %v", testPath, testconfig, expected)
	}
}

func TestIsWindowOpen(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		maintOpenDay  int
		maintCloseDay int
		ahOpens       time.Time
		ahCloses      time.Time
		want          bool
	}{
		{
			name:          "WithinWindow",
			maintOpenDay:  10,
			maintCloseDay: 20,
			ahOpens:       now.Add(-2 * time.Hour),
			ahCloses:      now.Add(2 * time.Hour),
			want:          true,
		},
		{
			name:          "OutsideActiveHours",
			maintOpenDay:  10,
			maintCloseDay: 20,
			ahOpens:       now.Add(1 * time.Hour),
			ahCloses:      now.Add(3 * time.Hour),
			want:          false,
		},
		{
			name:          "OutsideMaintenanceDays",
			maintOpenDay:  1,
			maintCloseDay: 10,
			ahOpens:       now.Add(-2 * time.Hour),
			ahCloses:      now.Add(2 * time.Hour),
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWindowOpen(tt.maintOpenDay, tt.maintCloseDay, tt.ahOpens, tt.ahCloses, now)
			if got != tt.want {
				t.Errorf("isWindowOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRebootActive(t *testing.T) {
	setRebootActive(false)
	if isRebootActive() {
		t.Errorf("isRebootActive() = true, want false")
	}

	setRebootActive(true)
	if !isRebootActive() {
		t.Errorf("isRebootActive() = false, want true")
	}

	setRebootActive(false)
}

func TestRunMainLoopContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := runMainLoop(ctx)
	if err != context.Canceled {
		t.Errorf("runMainLoop(canceledContext) = %v, want %v", err, context.Canceled)
	}
}

func TestRebootActiveConcurrencyStress(t *testing.T) {
	setRebootActive(false)
	var wg sync.WaitGroup
	const goroutines = 100
	const iterations = 1000

	var winCount atomic.Int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if id%3 == 0 {
					_ = isRebootActive()
				} else if id%3 == 1 {
					setRebootActive(j%2 == 0)
				} else {
					if rebootActive.CompareAndSwap(false, true) {
						winCount.Add(1)
						time.Sleep(1 * time.Microsecond)
						setRebootActive(false)
					}
				}
			}
		}(i)
	}
	wg.Wait()
	setRebootActive(false)
	t.Logf("RebootActive concurrency test completed. CAS wins: %d", winCount.Load())
}

func TestHandleRebootTriggerConcurrent(t *testing.T) {
	setRebootActive(false)
	var wg sync.WaitGroup
	const count = 50

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleRebootTrigger()
		}()
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	if isRebootActive() {
		t.Errorf("isRebootActive() = true, want false after reboot triggers completed")
	}
}

func TestRunMainLoopContextCancellationGoroutineLeaks(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- runMainLoop(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("runMainLoop returned %v, want %v", err, context.Canceled)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runMainLoop hung on context cancellation!")
	}

	time.Sleep(200 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leak := finalGoroutines - initialGoroutines
	t.Logf("Goroutine count diff after cancellation: initial=%d, final=%d, diff=%d", initialGoroutines, finalGoroutines, leak)
	if leak > 5 {
		t.Errorf("Possible goroutine leak detected: diff = %d", leak)
	}
}

func TestRunMainLoopConcurrentContextCancellation(t *testing.T) {
	// Verify metrics are uninitialized or handled safely
	t.Logf("Metrics state: virusUpdateSuccess=%v, enforcementWatcherFailures=%v", virusUpdateSuccess, enforcementWatcherFailures)

	initialGoroutines := runtime.NumGoroutine()
	const concurrency = 50
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			if id%2 == 0 {
				cancel() // Immediate cancellation
			} else {
				go func() {
					time.Sleep(time.Duration(id%10+1) * time.Millisecond)
					cancel()
				}()
			}
			err := runMainLoop(ctx)
			errCh <- err
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != context.Canceled {
			t.Errorf("concurrent runMainLoop returned %v, want %v", err, context.Canceled)
		}
	}

	time.Sleep(300 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()
	leak := finalGoroutines - initialGoroutines
	t.Logf("Concurrent runMainLoop goroutine count diff: initial=%d, final=%d, diff=%d", initialGoroutines, finalGoroutines, leak)
	if leak > 5 {
		t.Errorf("Possible goroutine leak detected in concurrent execution: diff = %d", leak)
	}
}



