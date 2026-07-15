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

// Package notification provides user notification messages.
package notification

import (
	"golang.org/x/net/context"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"gopkg.in/toast.v1"
)

// CleanNotifications deletes any active Cabbie notification messages.
func CleanNotifications(name string) error {
	if name == "" {
		name = appID
	}
	psCmd := fmt.Sprintf(`try { [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; [Windows.UI.Notifications.ToastNotificationManager]::History.Clear('%s') } catch {}`, name)
	cmd := exec.Command("PowerShell", "-ExecutionPolicy", "Bypass", "-Command", psCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
	return nil
}

// ToastNotification wraps toast.Notification to accept context.Context.
type ToastNotification struct {
	toast.Notification
}

// Push triggers the toast notification.
func (n *ToastNotification) Push(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return n.Notification.Push()
}

// NewRebootMessage returns a standard reboot message.
func NewRebootMessage(t time.Time) Notification {
	tf := t.Format(time.UnixDate)
	return &ToastNotification{
		Notification: toast.Notification{
			AppID:   appID,
			Title:   "Reboot Your Machine",
			Message: fmt.Sprintf("Reboot now to finish installing updates. Your machine will auto reboot at %s.", tf),
		},
	}
}

// RebootPopup returns a reboot warning popup message.
func RebootPopup(minutes int) Notification {
	return &ToastNotification{
		Notification: toast.Notification{
			AppID:   appID,
			Title:   "Force Reboot Soon",
			Message: fmt.Sprintf("To finish installing the newest updates, your machine will auto reboot in %d minutes.", minutes),
		},
	}
}

// NewAvailableUpdateMessage returns an available updates message.
func NewAvailableUpdateMessage() Notification {
	return &ToastNotification{
		Notification: toast.Notification{
			AppID:   appID,
			Title:   "Updates Available",
			Message: "New Windows updates are now available. Please install at your earliest convenience.",
		},
	}
}

// NewInstallingMessage returns an installing updates message.
func NewInstallingMessage() Notification {
	return &ToastNotification{
		Notification: toast.Notification{
			AppID:   appID,
			Title:   "Installing Updates",
			Message: "Cabbie is installing new updates.",
		},
	}
}
