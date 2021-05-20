// Copyright 2021 Google LLC
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

package cablib

const (
	/*
	 * System Events
	 */

	// EvtReboot indicates a reboot of the local system.
	EvtReboot = iota + 1000

	/*
	 * Internal Events
	 */

	// EvtUpdatesFound indicates that applicable updates were detected.
	EvtUpdatesFound = iota + 2000
	// EvtNoUpdates indicates that no applicable updates were detected.
	EvtNoUpdates
	// EvtServiceStarted indicates that the cabbie service has started.
	EvtServiceStarted
	// EvtServiceStarting indicates that the cabbie service is starting.
	EvtServiceStarting
	// EvtServiceStopped indicates that the cabbie service has stopped.
	EvtServiceStopped
	// EvtEnforcementChange indicates that cabbie has detected a change in one or more enforcement files.
	EvtEnforcementChange

	/*
	 * Errors
	 */

	// EvtErrMetricReport indicates a problem reporting metric data.
	EvtErrMetricReport = iota + 4000
	// EvtErrNotifications indicates a problem displaying notifications.
	EvtErrNotifications
	// EvtErrInstallFailure indicates a problem installing updates.
	EvtErrInstallFailure
	// EvtErrQueryFailure indicates a problem querying for updates.
	EvtErrQueryFailure
	// EvtErrMaintWindow indicates a problem with maintenance window configuration.
	EvtErrMaintWindow
)
