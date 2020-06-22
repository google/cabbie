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

// Package notification provides user notification messages.
package notification

import (
	"fmt"
	"time"
)

// NewRebootMessage returns a standard reboot message.
func NewRebootMessage(seconds int) string {
	t := time.Now().Add(time.Second * time.Duration(seconds)).Format(time.UnixDate)
	return fmt.Sprintf("Reboot now to finish installing updates. Your machine will auto reboot at %s", t)
}

// NewAvailableUpdateMessage returns an available updates message.
func NewAvailableUpdateMessage() string {
	return "New Windows updates are now available. Please install at your earliest convenience"
}

// NewInstallingMessage returns an installing updates message.
func NewInstallingMessage() string {
	return "Cabbie is installing new updates."
}

// CleanNotifications deletes any active Cabbie notification messages.
func CleanNotifications(name string) error {
	return nil
}

// NewNotification posts a notification message.
func NewNotification(service string, message string, id string) error {
	return nil
}
