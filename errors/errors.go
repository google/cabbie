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

// Package errors translates the API return from hex into a help string.
package errors

import "fmt"

// UpdateError represents an API hexadecimal return code
type UpdateError int64

// Update API errors collected from:
// https://docs.microsoft.com/en-us/windows/desktop/wua_sdk/wua-success-and-error-codes-
// https://docs.microsoft.com/en-us/windows/desktop/wua_sdk/wua-networking-error-codes-
const (
	SUCCESS                               UpdateError = 0x00000000
	TIME_OUT_ERRORS                       UpdateError = 0x80072EFE
	TIME_OUT_ERRORS2                      UpdateError = 0x80D02002
	ERROR_WINHTTP_CANNOT_CONNECT          UpdateError = 0x80072EFD
	WININET_E_TIMEOUT                     UpdateError = 0x80072EE2
	WU_S_SERVICE_STOP                     UpdateError = 0x00240001
	WU_S_SELFUPDATE                       UpdateError = 0x00240002
	WU_S_UPDATE_ERROR                     UpdateError = 0x00240003
	WU_S_MARKED_FOR_DISCONNECT            UpdateError = 0x00240004
	WU_S_REBOOT_REQUIRED                  UpdateError = 0x00240005
	WU_S_ALREADY_INSTALLED                UpdateError = 0x00240006
	WU_S_ALREADY_UNINSTALLED              UpdateError = 0x00240007
	WU_S_ALREADY_DOWNLOADED               UpdateError = 0x00240008
	WU_S_UH_INSTALLSTILLPENDING           UpdateError = 0x00242015
	WU_E_NO_SERVICE                       UpdateError = 0x80240001
	WU_E_MAX_CAPACITY_REACHED             UpdateError = 0x80240002
	WU_E_UNKNOWN_ID                       UpdateError = 0x80240003
	WU_E_NOT_INITIALIZED                  UpdateError = 0x80240004
	WU_E_RANGEOVERLAP                     UpdateError = 0x80240005
	WU_E_TOOMANYRANGES                    UpdateError = 0x80240006
	WU_E_INVALIDINDEX                     UpdateError = 0x80240007
	WU_E_ITEMNOTFOUND                     UpdateError = 0x80240008
	WU_E_OPERATIONINPROGRESS              UpdateError = 0x80240009
	WU_E_COULDNOTCANCEL                   UpdateError = 0x8024000A
	WU_E_CALL_CANCELLED                   UpdateError = 0x8024000B
	WU_E_NOOP                             UpdateError = 0x8024000C
	WU_E_XML_MISSINGDATA                  UpdateError = 0x8024000D
	WU_E_XML_INVALID                      UpdateError = 0x8024000E
	WU_E_CYCLE_DETECTED                   UpdateError = 0x8024000F
	WU_E_TOO_DEEP_RELATION                UpdateError = 0x80240010
	WU_E_INVALID_RELATIONSHIP             UpdateError = 0x80240011
	WU_E_REG_VALUE_INVALID                UpdateError = 0x80240012
	WU_E_DUPLICATE_ITEM                   UpdateError = 0x80240013
	WU_E_INVALID_INSTALL_REQUESTED        UpdateError = 0x80240014
	WU_E_INSTALL_NOT_ALLOWED              UpdateError = 0x80240016
	WU_E_NOT_APPLICABLE                   UpdateError = 0x80240017
	WU_E_NO_USERTOKEN                     UpdateError = 0x80240018
	WU_E_EXCLUSIVE_INSTALL_CONFLICT       UpdateError = 0x80240019
	WU_E_POLICY_NOT_SET                   UpdateError = 0x8024001A
	WU_E_SELFUPDATE_IN_PROGRESS           UpdateError = 0x8024001B
	WU_E_INVALID_UPDATE                   UpdateError = 0x8024001D
	WU_E_SERVICE_STOP                     UpdateError = 0x8024001E
	WU_E_NO_CONNECTION                    UpdateError = 0x8024001F
	WU_E_NO_INTERACTIVE_USER              UpdateError = 0x80240020
	WU_E_TIME_OUT                         UpdateError = 0x80240021
	WU_E_ALL_UPDATES_FAILED               UpdateError = 0x80240022
	WU_E_EULAS_DECLINED                   UpdateError = 0x80240023
	WU_E_NO_UPDATE                        UpdateError = 0x80240024
	WU_E_USER_ACCESS_DISABLED             UpdateError = 0x80240025
	WU_E_INVALID_UPDATE_TYPE              UpdateError = 0x80240026
	WU_E_URL_TOO_LONG                     UpdateError = 0x80240027
	WU_E_UNINSTALL_NOT_ALLOWED            UpdateError = 0x80240028
	WU_E_INVALID_PRODUCT_LICENSE          UpdateError = 0x80240029
	WU_E_MISSING_HANDLER                  UpdateError = 0x8024002A
	WU_E_LEGACYSERVER                     UpdateError = 0x8024002B
	WU_E_BIN_SOURCE_ABSENT                UpdateError = 0x8024002C
	WU_E_SOURCE_ABSENT                    UpdateError = 0x8024002D
	WU_E_WU_DISABLED                      UpdateError = 0x8024002E
	WU_E_CALL_CANCELLED_BY_POLICY         UpdateError = 0x8024002F
	WU_E_INVALID_PROXY_SERVER             UpdateError = 0x80240030
	WU_E_INVALID_FILE                     UpdateError = 0x80240031
	WU_E_INVALID_CRITERIA                 UpdateError = 0x80240032
	WU_E_EULA_UNAVAILABLE                 UpdateError = 0x80240033
	WU_E_DOWNLOAD_FAILED                  UpdateError = 0x80240034
	WU_E_UPDATE_NOT_PROCESSED             UpdateError = 0x80240035
	WU_E_INVALID_OPERATION                UpdateError = 0x80240036
	WU_E_NOT_SUPPORTED                    UpdateError = 0x80240037
	WU_E_TOO_MANY_RESYNC                  UpdateError = 0x80240039
	WU_E_NO_SERVER_CORE_SUPPORT           UpdateError = 0x80240040
	WU_E_SYSPREP_IN_PROGRESS              UpdateError = 0x80240041
	WU_E_UNKNOWN_SERVICE                  UpdateError = 0x80240042
	WU_E_NO_UI_SUPPORT                    UpdateError = 0x80240043
	WU_E_PER_MACHINE_UPDATE_ACCESS_DENIED UpdateError = 0x80240044
	WU_E_UNSUPPORTED_SEARCHSCOPE          UpdateError = 0x80240045
	WU_E_BAD_FILE_URL                     UpdateError = 0x80240046
	WU_E_NOTSUPPORTED                     UpdateError = 0x80240047
	WU_E_INVALID_NOTIFICATION_INFO        UpdateError = 0x80240048
	WU_E_OUTOFRANGE                       UpdateError = 0x80240049
	WU_E_SETUP_IN_PROGRESS                UpdateError = 0x8024004A
	WU_E_UNEXPECTED                       UpdateError = 0x80240FFF
	WU_E_WINHTTP_INVALID_FILE             UpdateError = 0x80240038
	WU_E_DS_UNKNOWNSERVICE                UpdateError = 0x80248014
	WU_E_PT_ECP_SUCCEEDED_WITH_ERRORS     UpdateError = 0x8024402F
	WU_E_PT_HTTP_STATUS_BAD_REQUEST       UpdateError = 0x80244016
	WU_E_PT_HTTP_STATUS_DENIED            UpdateError = 0x80244017
	WU_E_PT_HTTP_STATUS_FORBIDDEN         UpdateError = 0x80244018
	WU_E_PT_HTTP_STATUS_NOT_FOUND         UpdateError = 0x80244019
	WU_E_PT_HTTP_STATUS_BAD_METHOD        UpdateError = 0x8024401A
	WU_E_PT_HTTP_STATUS_PROXY_AUTH_REQ    UpdateError = 0x8024401B
	WU_E_PT_HTTP_STATUS_REQUEST_TIMEOUT   UpdateError = 0x8024401C
	WU_E_PT_HTTP_STATUS_CONFLICT          UpdateError = 0x8024401D
	WU_E_PT_HTTP_STATUS_GONE              UpdateError = 0x8024401E
	WU_E_PT_HTTP_STATUS_SERVER_ERROR      UpdateError = 0x8024401F
	WU_E_PT_HTTP_STATUS_NOT_SUPPORTED     UpdateError = 0x80244020
	WU_E_PT_HTTP_STATUS_BAD_GATEWAY       UpdateError = 0x80244021
	WU_E_PT_HTTP_STATUS_SERVICE_UNAVAIL   UpdateError = 0x80244022
	WU_E_PT_HTTP_STATUS_GATEWAY_TIMEOUT   UpdateError = 0x80244023
	WU_E_PT_HTTP_STATUS_VERSION_NOT_SUP   UpdateError = 0x80244024
	WU_E_PT_HTTP_STATUS_NOT_MAPPED        UpdateError = 0x8024402B
	WU_E_PT_WINHTTP_NAME_NOT_RESOLVED     UpdateError = 0x8024402C
	TRY_AGAIN_ERROR                       UpdateError = 0x80240438
	TIME_VERIFICATION                     UpdateError = 0x80072F8F
	EXCEPTION_OCCURRED                    UpdateError = 0x8024500C
)

// ErrorDesc gets the help string related to a hex error.
func (ue UpdateError) ErrorDesc() string {
	switch ue {
	case SUCCESS:
		return `Update operation was successful.`
	case TIME_OUT_ERRORS:
		return `The operation timed out.`
	case TIME_OUT_ERRORS2:
		return `The operation timed out.`
	case ERROR_WINHTTP_CANNOT_CONNECT:
		return `The attempt to connect to the server failed. / Internet cannot connect.`
	case WININET_E_TIMEOUT:
		return `The operation timed out`
	case WU_S_SERVICE_STOP:
		return `WUA was stopped successfully.`
	case WU_S_SELFUPDATE:
		return `WUA updated itself.`
	case WU_S_UPDATE_ERROR:
		return `The operation completed successfully but errors occurred applying the updates.`
	case WU_S_MARKED_FOR_DISCONNECT:
		return `A callback was marked to be disconnected later because the request to disconnect the operation came while a callback was executing.`
	case WU_S_REBOOT_REQUIRED:
		return `The system must be restarted to complete installation of the update.`
	case WU_S_ALREADY_INSTALLED:
		return `The update to be installed is already installed on the system.`
	case WU_S_ALREADY_UNINSTALLED:
		return `The update to be removed is not installed on the system.`
	case WU_S_ALREADY_DOWNLOADED:
		return `The update to be downloaded has already been downloaded.`
	case WU_S_UH_INSTALLSTILLPENDING:
		return `The installation operation for the update is still in progress.`
	case WU_E_NO_SERVICE:
		return `WUA was unable to provide the service.`
	case WU_E_MAX_CAPACITY_REACHED:
		return `The maximum capacity of the service was exceeded.`
	case WU_E_UNKNOWN_ID:
		return `WUA cannot find an ID.`
	case WU_E_NOT_INITIALIZED:
		return `The object could not be initialized.`
	case WU_E_RANGEOVERLAP:
		return `The update handler requested a byte range overlapping a previously requested range.`
	case WU_E_TOOMANYRANGES:
		return `The requested number of byte ranges exceeds the maximum number (231 - 1).`
	case WU_E_INVALIDINDEX:
		return `The index to a collection was invalid.`
	case WU_E_ITEMNOTFOUND:
		return `The key for the item queried could not be found.`
	case WU_E_OPERATIONINPROGRESS:
		return `Another conflicting operation was in progress. Some operations such as installation cannot be performed twice simultaneously.`
	case WU_E_COULDNOTCANCEL:
		return `Cancellation of the operation was not allowed.`
	case WU_E_CALL_CANCELLED:
		return `Operation was cancelled.`
	case WU_E_NOOP:
		return `No operation was required.`
	case WU_E_XML_MISSINGDATA:
		return `WUA could not find required information in the update's XML data.`
	case WU_E_XML_INVALID:
		return `WUA found invalid information in the update's XML data.`
	case WU_E_CYCLE_DETECTED:
		return `Circular update relationships were detected in the metadata.`
	case WU_E_TOO_DEEP_RELATION:
		return `Update relationships too deep to evaluate were evaluated.`
	case WU_E_INVALID_RELATIONSHIP:
		return `An invalid update relationship was detected.`
	case WU_E_REG_VALUE_INVALID:
		return `An invalid registry value was read.`
	case WU_E_DUPLICATE_ITEM:
		return `Operation tried to add a duplicate item to a list.`
	case WU_E_INVALID_INSTALL_REQUESTED:
		return `Updates that are requested for install are not installable by the caller.`
	case WU_E_INSTALL_NOT_ALLOWED:
		return `Operation tried to install while another installation was in progress or the system was pending a mandatory restart.`
	case WU_E_NOT_APPLICABLE:
		return `Operation was not performed because there are no applicable updates.`
	case WU_E_NO_USERTOKEN:
		return `Operation failed because a required user token is missing.`
	case WU_E_EXCLUSIVE_INSTALL_CONFLICT:
		return `An exclusive update can't be installed with other updates at the same time.`
	case WU_E_POLICY_NOT_SET:
		return `A policy value was not set.`
	case WU_E_SELFUPDATE_IN_PROGRESS:
		return `The operation could not be performed because the Windows Update Agent is self-updating.`
	case WU_E_INVALID_UPDATE:
		return `An update contains invalid metadata.`
	case WU_E_SERVICE_STOP:
		return `Operation did not complete because the service or system was being shut down.`
	case WU_E_NO_CONNECTION:
		return `Operation did not complete because the network connection was unavailable.`
	case WU_E_NO_INTERACTIVE_USER:
		return `Operation did not complete because there is no logged-on interactive user.`
	case WU_E_TIME_OUT:
		return `Operation did not complete because it timed out.`
	case WU_E_ALL_UPDATES_FAILED:
		return `Operation failed for all the updates.`
	case WU_E_EULAS_DECLINED:
		return `The license terms for all updates were declined.`
	case WU_E_NO_UPDATE:
		return `There are no updates.`
	case WU_E_USER_ACCESS_DISABLED:
		return `Group Policy settings prevented access to Windows Update.`
	case WU_E_INVALID_UPDATE_TYPE:
		return `The type of update is invalid.`
	case WU_E_URL_TOO_LONG:
		return `The URL exceeded the maximum length.`
	case WU_E_UNINSTALL_NOT_ALLOWED:
		return `The update could not be uninstalled because the request did not originate from a Windows Server Update Services (WSUS) server.`
	case WU_E_INVALID_PRODUCT_LICENSE:
		return `Search may have missed some updates before there is an unlicensed application on the system.`
	case WU_E_MISSING_HANDLER:
		return `A component required to detect applicable updates was missing.`
	case WU_E_LEGACYSERVER:
		return `An operation did not complete because it requires a newer version of server.`
	case WU_E_BIN_SOURCE_ABSENT:
		return `A delta-compressed update could not be installed because it required the source.`
	case WU_E_SOURCE_ABSENT:
		return `A full-file update could not be installed because it required the source.`
	case WU_E_WU_DISABLED:
		return `Access to an unmanaged server is not allowed.`
	case WU_E_CALL_CANCELLED_BY_POLICY:
		return `Operation did not complete because the DisableWindowsUpdateAccess policy was set in the registry.`
	case WU_E_INVALID_PROXY_SERVER:
		return `The format of the proxy list was invalid.`
	case WU_E_INVALID_FILE:
		return `The file is in the wrong format.`
	case WU_E_INVALID_CRITERIA:
		return `The search criteria string was invalid.`
	case WU_E_EULA_UNAVAILABLE:
		return `License terms could not be downloaded.`
	case WU_E_DOWNLOAD_FAILED:
		return `Update failed to download.`
	case WU_E_UPDATE_NOT_PROCESSED:
		return `The update was not processed.`
	case WU_E_INVALID_OPERATION:
		return `The object's current state did not allow the operation.`
	case WU_E_NOT_SUPPORTED:
		return `The functionality for the operation is not supported.`
	case WU_E_TOO_MANY_RESYNC:
		return `Agent is asked by server to resync too many times.`
	case WU_E_NO_SERVER_CORE_SUPPORT:
		return `The WUA API method does not run on the server core installation.`
	case WU_E_SYSPREP_IN_PROGRESS:
		return `Service is not available while sysprep is running.`
	case WU_E_UNKNOWN_SERVICE:
		return `The update service is no longer registered with automatic updates.`
	case WU_E_NO_UI_SUPPORT:
		return `No support for the WUA user interface.`
	case WU_E_PER_MACHINE_UPDATE_ACCESS_DENIED:
		return `Only administrators can perform this operation on per-computer updates.`
	case WU_E_UNSUPPORTED_SEARCHSCOPE:
		return `A search was attempted with a scope that is not currently supported for this type of search.`
	case WU_E_BAD_FILE_URL:
		return `The URL does not point to a file.`
	case WU_E_NOTSUPPORTED:
		return `The operation requested is not supported.`
	case WU_E_INVALID_NOTIFICATION_INFO:
		return `The featured update notification info returned by the server is invalid.`
	case WU_E_OUTOFRANGE:
		return `The data is out of range.`
	case WU_E_SETUP_IN_PROGRESS:
		return `WUA operations are not available while operating system setup is running.`
	case WU_E_UNEXPECTED:
		return `An operation failed due to reasons not covered by another error code.`
	case WU_E_WINHTTP_INVALID_FILE:
		return `The downloaded file has an unexpected content type.`
	case WU_E_DS_UNKNOWNSERVICE:
		return `Attempt to register an unknown service.`
	case WU_E_PT_ECP_SUCCEEDED_WITH_ERRORS:
		return `External cab file processing completed with some errors`
	case WU_E_PT_HTTP_STATUS_BAD_REQUEST:
		return `Same as HTTP status 400 – The server could not process the request due to invalid syntax.`
	case WU_E_PT_HTTP_STATUS_DENIED:
		return `Same as HTTP status 401 – The requested resource requires user authentication.`
	case WU_E_PT_HTTP_STATUS_FORBIDDEN:
		return `Same as HTTP status 403 – Server understood the request, but declines to fulfill it.`
	case WU_E_PT_HTTP_STATUS_NOT_FOUND:
		return `Same as HTTP status 404 – The server cannot find the requested URI (Uniform Resource Identifier).`
	case WU_E_PT_HTTP_STATUS_BAD_METHOD:
		return `Same as HTTP status 405 – The HTTP method is not allowed.`
	case WU_E_PT_HTTP_STATUS_PROXY_AUTH_REQ:
		return `Same as HTTP status 407 – Proxy authentication is required.`
	case WU_E_PT_HTTP_STATUS_REQUEST_TIMEOUT:
		return `Same as HTTP status 408 – The server timed out waiting for the request.`
	case WU_E_PT_HTTP_STATUS_CONFLICT:
		return `Same as HTTP status 409 – The request was not completed due to a conflict with the current state of the resource.`
	case WU_E_PT_HTTP_STATUS_GONE:
		return `Same as HTTP status 410 – Requested resource is no longer available at the server.`
	case WU_E_PT_HTTP_STATUS_SERVER_ERROR:
		return `Same as HTTP status 500 – An error internal to the server prevented fulfilling the request.`
	case WU_E_PT_HTTP_STATUS_NOT_SUPPORTED:
		return `Same as HTTP status 501 – Server does not support the functionality required to fulfill the request.`
	case WU_E_PT_HTTP_STATUS_BAD_GATEWAY:
		return `Same as HTTP status 502 – The server, while acting as a gateway or proxy, received an invalid response from the upstream server it accessed in attempting to fulfill the request.`
	case WU_E_PT_HTTP_STATUS_SERVICE_UNAVAIL:
		return `Same as HTTP status 503 – The service is temporarily overloaded.`
	case WU_E_PT_HTTP_STATUS_GATEWAY_TIMEOUT:
		return `Same as HTTP status 504 – The request was timed out waiting for a gateway.`
	case WU_E_PT_HTTP_STATUS_VERSION_NOT_SUP:
		return `Same as HTTP status 505 – The server does not support the HTTP protocol version used for the request.`
	case WU_E_PT_HTTP_STATUS_NOT_MAPPED:
		return `The request could not be completed and the reason did not correspond to any of the WU_E_PT_HTTP_* error codes.`
	case WU_E_PT_WINHTTP_NAME_NOT_RESOLVED:
		return `Same as ERROR_WINHTTP_NAME_NOT_RESOLVED - The proxy server or target server name cannot be resolved.`
	case TRY_AGAIN_ERROR:
		return `There were some problems installing updates, but we'll try again later.`
	case TIME_VERIFICATION:
		return `Incorrect date, time and timezone settings on the computer.`
	case EXCEPTION_OCCURRED:
		return `General COM exception occurred, usually caused by a misconfigured registry setting.`
	default:
		return fmt.Sprintf("Unknown error: 0x%X", int64(ue))
	}
}

//ErrorName converts the hex error code into the error name.
func (ue UpdateError) ErrorName() string {
	switch ue {
	case SUCCESS:
		return `SUCCESS`
	case TIME_OUT_ERRORS:
		return `TIME_OUT_ERRORS`
	case TIME_OUT_ERRORS2:
		return `TIME_OUT_ERRORS`
	case ERROR_WINHTTP_CANNOT_CONNECT:
		return `ERROR_WINHTTP_CANNOT_CONNECT`
	case WININET_E_TIMEOUT:
		return `WININET_E_TIMEOUT`
	case WU_S_SERVICE_STOP:
		return `WU_S_SERVICE_STOP`
	case WU_S_SELFUPDATE:
		return `WU_S_SELFUPDATE`
	case WU_S_UPDATE_ERROR:
		return `WU_S_UPDATE_ERROR`
	case WU_S_MARKED_FOR_DISCONNECT:
		return `WU_S_MARKED_FOR_DISCONNECT`
	case WU_S_REBOOT_REQUIRED:
		return `WU_S_REBOOT_REQUIRED`
	case WU_S_ALREADY_INSTALLED:
		return `WU_S_ALREADY_INSTALLED`
	case WU_S_ALREADY_UNINSTALLED:
		return `WU_S_ALREADY_UNINSTALLED`
	case WU_S_ALREADY_DOWNLOADED:
		return `WU_S_ALREADY_DOWNLOADED`
	case WU_S_UH_INSTALLSTILLPENDING:
		return `WU_S_UH_INSTALLSTILLPENDING`
	case WU_E_NO_SERVICE:
		return `WU_E_NO_SERVICE`
	case WU_E_MAX_CAPACITY_REACHED:
		return `WU_E_MAX_CAPACITY_REACHED`
	case WU_E_UNKNOWN_ID:
		return `WU_E_UNKNOWN_ID`
	case WU_E_NOT_INITIALIZED:
		return `WU_E_NOT_INITIALIZED`
	case WU_E_RANGEOVERLAP:
		return `WU_E_RANGEOVERLAP`
	case WU_E_TOOMANYRANGES:
		return `WU_E_TOOMANYRANGES`
	case WU_E_INVALIDINDEX:
		return `WU_E_INVALIDINDEX`
	case WU_E_ITEMNOTFOUND:
		return `WU_E_ITEMNOTFOUND`
	case WU_E_OPERATIONINPROGRESS:
		return `WU_E_OPERATIONINPROGRESS`
	case WU_E_COULDNOTCANCEL:
		return `WU_E_COULDNOTCANCEL`
	case WU_E_CALL_CANCELLED:
		return `WU_E_CALL_CANCELLED`
	case WU_E_NOOP:
		return `WU_E_NOOP`
	case WU_E_XML_MISSINGDATA:
		return `WU_E_XML_MISSINGDATA`
	case WU_E_XML_INVALID:
		return `WU_E_XML_INVALID`
	case WU_E_CYCLE_DETECTED:
		return `WU_E_CYCLE_DETECTED`
	case WU_E_TOO_DEEP_RELATION:
		return `WU_E_TOO_DEEP_RELATION`
	case WU_E_INVALID_RELATIONSHIP:
		return `WU_E_INVALID_RELATIONSHIP`
	case WU_E_REG_VALUE_INVALID:
		return `WU_E_REG_VALUE_INVALID`
	case WU_E_DUPLICATE_ITEM:
		return `WU_E_DUPLICATE_ITEM`
	case WU_E_INVALID_INSTALL_REQUESTED:
		return `WU_E_INVALID_INSTALL_REQUESTED`
	case WU_E_INSTALL_NOT_ALLOWED:
		return `WU_E_INSTALL_NOT_ALLOWED`
	case WU_E_NOT_APPLICABLE:
		return `WU_E_NOT_APPLICABLE`
	case WU_E_NO_USERTOKEN:
		return `WU_E_NO_USERTOKEN`
	case WU_E_EXCLUSIVE_INSTALL_CONFLICT:
		return `WU_E_EXCLUSIVE_INSTALL_CONFLICT`
	case WU_E_POLICY_NOT_SET:
		return `WU_E_POLICY_NOT_SET`
	case WU_E_SELFUPDATE_IN_PROGRESS:
		return `WU_E_SELFUPDATE_IN_PROGRESS`
	case WU_E_INVALID_UPDATE:
		return `WU_E_INVALID_UPDATE`
	case WU_E_SERVICE_STOP:
		return `WU_E_SERVICE_STOP`
	case WU_E_NO_CONNECTION:
		return `WU_E_NO_CONNECTION`
	case WU_E_NO_INTERACTIVE_USER:
		return `WU_E_NO_INTERACTIVE_USER`
	case WU_E_TIME_OUT:
		return `WU_E_TIME_OUT`
	case WU_E_ALL_UPDATES_FAILED:
		return `WU_E_ALL_UPDATES_FAILED`
	case WU_E_EULAS_DECLINED:
		return `WU_E_EULAS_DECLINED`
	case WU_E_NO_UPDATE:
		return `WU_E_NO_UPDATE`
	case WU_E_USER_ACCESS_DISABLED:
		return `WU_E_USER_ACCESS_DISABLED`
	case WU_E_INVALID_UPDATE_TYPE:
		return `WU_E_INVALID_UPDATE_TYPE`
	case WU_E_URL_TOO_LONG:
		return `WU_E_URL_TOO_LONG`
	case WU_E_UNINSTALL_NOT_ALLOWED:
		return `WU_E_UNINSTALL_NOT_ALLOWED`
	case WU_E_INVALID_PRODUCT_LICENSE:
		return `WU_E_INVALID_PRODUCT_LICENSE`
	case WU_E_MISSING_HANDLER:
		return `WU_E_MISSING_HANDLER`
	case WU_E_LEGACYSERVER:
		return `WU_E_LEGACYSERVER`
	case WU_E_BIN_SOURCE_ABSENT:
		return `WU_E_BIN_SOURCE_ABSENT`
	case WU_E_SOURCE_ABSENT:
		return `WU_E_SOURCE_ABSENT`
	case WU_E_WU_DISABLED:
		return `WU_E_WU_DISABLED`
	case WU_E_CALL_CANCELLED_BY_POLICY:
		return `WU_E_CALL_CANCELLED_BY_POLICY`
	case WU_E_INVALID_PROXY_SERVER:
		return `WU_E_INVALID_PROXY_SERVER`
	case WU_E_INVALID_FILE:
		return `WU_E_INVALID_FILE`
	case WU_E_INVALID_CRITERIA:
		return `WU_E_INVALID_CRITERIA`
	case WU_E_EULA_UNAVAILABLE:
		return `WU_E_EULA_UNAVAILABLE`
	case WU_E_DOWNLOAD_FAILED:
		return `WU_E_DOWNLOAD_FAILED`
	case WU_E_UPDATE_NOT_PROCESSED:
		return `WU_E_UPDATE_NOT_PROCESSED`
	case WU_E_INVALID_OPERATION:
		return `WU_E_INVALID_OPERATION`
	case WU_E_NOT_SUPPORTED:
		return `WU_E_NOT_SUPPORTED`
	case WU_E_TOO_MANY_RESYNC:
		return `WU_E_TOO_MANY_RESYNC`
	case WU_E_NO_SERVER_CORE_SUPPORT:
		return `WU_E_NO_SERVER_CORE_SUPPORT`
	case WU_E_SYSPREP_IN_PROGRESS:
		return `WU_E_SYSPREP_IN_PROGRESS`
	case WU_E_UNKNOWN_SERVICE:
		return `WU_E_UNKNOWN_SERVICE`
	case WU_E_NO_UI_SUPPORT:
		return `WU_E_NO_UI_SUPPORT`
	case WU_E_PER_MACHINE_UPDATE_ACCESS_DENIED:
		return `WU_E_PER_MACHINE_UPDATE_ACCESS_DENIED`
	case WU_E_UNSUPPORTED_SEARCHSCOPE:
		return `WU_E_UNSUPPORTED_SEARCHSCOPE`
	case WU_E_BAD_FILE_URL:
		return `WU_E_BAD_FILE_URL`
	case WU_E_NOTSUPPORTED:
		return `WU_E_NOTSUPPORTED`
	case WU_E_INVALID_NOTIFICATION_INFO:
		return `WU_E_INVALID_NOTIFICATION_INFO`
	case WU_E_OUTOFRANGE:
		return `WU_E_OUTOFRANGE`
	case WU_E_SETUP_IN_PROGRESS:
		return `WU_E_SETUP_IN_PROGRESS`
	case WU_E_UNEXPECTED:
		return `WU_E_UNEXPECTED`
	case WU_E_WINHTTP_INVALID_FILE:
		return `WU_E_WINHTTP_INVALID_FILE`
	case WU_E_DS_UNKNOWNSERVICE:
		return `WU_E_DS_UNKNOWNSERVICE`
	case WU_E_PT_ECP_SUCCEEDED_WITH_ERRORS:
		return `WU_E_PT_ECP_SUCCEEDED_WITH_ERRORS`
	case WU_E_PT_HTTP_STATUS_BAD_REQUEST:
		return `WU_E_PT_HTTP_STATUS_BAD_REQUEST`
	case WU_E_PT_HTTP_STATUS_DENIED:
		return `WU_E_PT_HTTP_STATUS_DENIED`
	case WU_E_PT_HTTP_STATUS_FORBIDDEN:
		return `WU_E_PT_HTTP_STATUS_FORBIDDEN`
	case WU_E_PT_HTTP_STATUS_NOT_FOUND:
		return `WU_E_PT_HTTP_STATUS_NOT_FOUND`
	case WU_E_PT_HTTP_STATUS_BAD_METHOD:
		return `WU_E_PT_HTTP_STATUS_BAD_METHOD`
	case WU_E_PT_HTTP_STATUS_PROXY_AUTH_REQ:
		return `WU_E_PT_HTTP_STATUS_PROXY_AUTH_REQ`
	case WU_E_PT_HTTP_STATUS_REQUEST_TIMEOUT:
		return `WU_E_PT_HTTP_STATUS_REQUEST_TIMEOUT`
	case WU_E_PT_HTTP_STATUS_CONFLICT:
		return `WU_E_PT_HTTP_STATUS_CONFLICT`
	case WU_E_PT_HTTP_STATUS_GONE:
		return `WU_E_PT_HTTP_STATUS_GONE`
	case WU_E_PT_HTTP_STATUS_SERVER_ERROR:
		return `WU_E_PT_HTTP_STATUS_SERVER_ERROR`
	case WU_E_PT_HTTP_STATUS_NOT_SUPPORTED:
		return `WU_E_PT_HTTP_STATUS_NOT_SUPPORTED`
	case WU_E_PT_HTTP_STATUS_BAD_GATEWAY:
		return `WU_E_PT_HTTP_STATUS_BAD_GATEWAY`
	case WU_E_PT_HTTP_STATUS_SERVICE_UNAVAIL:
		return `WU_E_PT_HTTP_STATUS_SERVICE_UNAVAIL`
	case WU_E_PT_HTTP_STATUS_GATEWAY_TIMEOUT:
		return `WU_E_PT_HTTP_STATUS_GATEWAY_TIMEOUT`
	case WU_E_PT_HTTP_STATUS_VERSION_NOT_SUP:
		return `WU_E_PT_HTTP_STATUS_VERSION_NOT_SUP`
	case WU_E_PT_HTTP_STATUS_NOT_MAPPED:
		return `WU_E_PT_HTTP_STATUS_NOT_MAPPED`
	case WU_E_PT_WINHTTP_NAME_NOT_RESOLVED:
		return `WU_E_PT_WINHTTP_NAME_NOT_RESOLVED`
	case TRY_AGAIN_ERROR:
		return `TRY_AGAIN_ERROR`
	case TIME_VERIFICATION:
		return `TIME_VERIFICATION`
	case EXCEPTION_OCCURRED:
		return `EXCEPTION_OCCURRED`
	default:
		return ``
	}
}

func (ue UpdateError) String() string {
	return fmt.Sprintf("[%s] %s", ue.ErrorName(), ue.ErrorDesc())
}
