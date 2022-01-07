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

/*
 * System Events
 */
const (
	// EvtReboot indicates a reboot of the local system.
	EvtReboot = iota + 1000
)

/*
 * Internal Events
 */
const (
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
	// EvtDownload indicates that cabbie is downloading updates.
	EvtDownload
	// EvtInstall indicates that cabbie is installing updates.
	EvtInstall
	// EvtInstallSuccess indicates that cabbie successfully installed updates.
	EvtInstallSuccess
	// EvtSearch indicates that cabbie is searching for updates.
	EvtSearch
	// EvtRebootRequired indicates that the system is in need of a reboot.
	EvtRebootRequired
	// EvtUpdateSkip indicates that the cabbie is skipping detected updates.
	EvtUpdateSkip
	// EvtHistory indicates that cabbie is performing a history query.
	EvtHistory
	// EvtHide indicates that cabbie is attempting to hide updates.
	EvtHide
	// EvtList indicates that cabbie is performing an update list query.
	EvtList
	// EvtUnhide indicates that cabbie is attempting to unhide updates.
	EvtUnhide
	// EvtSvcInstall events relate to cabbie service (un)installation
	EvtSvcInstall
	// EvtMisc indicates that an uncategorized internal event.
	EvtMisc
)

/*
 * Errors
 */
const (
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
	// EvtErrUpdateScript indicates a problem with the pre/post update scripts.
	EvtErrUpdateScript
	// EvtErrDownloadFailure indicates that cabbie had a problem downloading updates.
	EvtErrDownloadFailure
	// EvtErrPowerMgmt indicates that cabbie had a problem managing device power state / rebooting.
	EvtErrPowerMgmt
	// EvtErrHistory indicates that cabbie had a problem querying update history.
	EvtErrHistory
	// EvtErrHide indicates that cabbie had a problem hiding updates.
	EvtErrHide
	// EvtErrUnhide indicates that cabbie had a problem unhiding updates
	EvtErrUnhide
	// EvtErrService indicates a problem with the Cabbie system service
	EvtErrService
	// EvtErrConfig indicates a problem with Cabbie configuration
	EvtErrConfig
	// EvtErrEnforcement indicates a problem with Cabbie's enforcement files
	EvtErrEnforcement
	// EvtErrSvcInstall indicates a problem during service (un)installation
	EvtErrSvcInstall
	// EvtErrMisc indicates a miscellaneous internal error condition.
	EvtErrMisc
)
