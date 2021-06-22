# Cabbie

[![Go Tests](https://github.com/google/cabbie/workflows/Go%20Tests/badge.svg)](https://github.com/google/cabbie/actions?query=workflow%3A%22Go+Tests%22)

## Overview

Cabbie is a standalone Go binary that utilizes the Windows Update Agent API to
search, download, and install Windows Updates. Cabbie can be configured to
connect to the public Windows Updates or to a local WSUS server.

The cabbie client has multiple flags, subcommands and subcommand arguments. For
an up-to-date list type `cabbie help` which will list command help.

Similarly, `cabbie flags` will show available flags.

## Getting Started

Download the repository and run `go build C:\Path\to\cabbie\src`

Install any missing imports with `go get <URL>`

## Configuration Options

These options can be configured using the registry key at
`HKLM:\SOFTWARE\Google\Cabbie`

Setting            | Registry Type | Default Setting                                      | Description
------------------ | ------------- | ---------------------------------------------------- | -----------
------------------ | ------------- | ------------------                                   | ------------------
WSUSServers        | REG_MULTI_SZ  | nil                                                  | List of WSUS servers to connect to instead of Microsoft updates.
RequiredCategories | REG_MULTI_SZ  | Critical Updates Definition Updates Security Updates | List of Update categories that an update must contain at least one of to be automatically installed.
UpdateDrivers      | REG_DWORD     | 0                                                    | Allow Cabbie to install available drivers.
UpdateVirusDef     | REG_DWORD     | 1                                                    | Allow Cabbie to install updated virus definitions every 30 minutes.
EnableThirdParty   | REG_DWORD     | 0                                                    | Allow Cabbie to check for third party software updates such off MSFT Office and Adobe.
RebootDelay        | REG_DWORD     | 21600                                                | Time in seconds for Cabbie to wait before force rebooting a machine to finalize update installation.
Deadline           | REG_DWORD     | 14                                                   | Number of days before Cabbie will force install an available update that matches the required categories. Set to "0" to disable this option.
NotifyAvailable    | REG_DWORD     | 1                                                    | If enabled Cabbie will send a notification when new required updates are available to be installed.
AukeraEnabled      | REG_DWORD     | 0                                                    | Enable Cabbie to use the open source Aukera maintenance window manager.
AukeraPort         | REG_DWORD     | 9119                                                 | LocalHost port to check against for Aukera maintenance windows.
AukeraName         | REG_SZ        | Cabbie                                               | Aukera maintenance window label to query for to determine if a maintenance window is currently open.
ScriptTimeout      | REG_DWORD     | 10                                                   | Pre/Post Update script timeout in minutes.

### Pre/Post Update script execution

Cabbie has the ability to run predefined scripts before and after each Windows
update install session. Each script is executed with a configurable timeout
(default 10 minutes) and will be automatically stopped if the timeout is
reached.

The scripts will need to be PowerShell scripts named `PreUpdate.ps1` and/or
`PostUpdate.ps1` located in the Cabbie root directory (`C:\Program
Files\Google\Cabbie`).

**PreUpdate.ps1**: This script will be executed once before the fist update in
the collection is download and installed.

**PostUpdate.ps1**: This script will be executed after the last update in the
collection is installed.

## Command-line Usage

`cabbie.exe <flags> <subcommand> <subcommand args>`

### List

Queries Microsoft for Windows Updates that are available to the device.

`cabbie list`

### Install

Searches, downloads, and installs updates from Microsoft or a configured local
WSUS.

Install all required updates (Default: security and critical updates):

`cabbie install`

Update available drivers:

`cabbie install -drivers`

Update virus definitions:

`cabbie install -virus_def`

Install specific update KBs:

`cabbie install -kbs="1234513,98765432"`

Install all applicable updates:

`cabbie install -all`

### History

Retrieves the recorded history of installed updates.

`cabbie history`

### Hide

Hides or unhides an update from installation.

Hide a KB:

`cabbie hide --kb="1234513"`

Make an update available for install:

`cabbie hide --unhide --kb="1234513"`

### Service

Manage the installation status of the Cabbie service.

Install Cabbie service:

`cabbie service --install`

Uninstall Cabbie service:

`cabbie service --uninstall`

## Service Usage

Cabbie can run as a Windows Service to enable constant update and reboot
management.

The basic steps to install the Cabbie service:

1.  Compile the Cabbie binary using `go build C:\Path\to\cabbie\src`.
2.  Copy the built `cabbie.exe` binary to the folder `C:\Program
    Files\Google\Cabbie`.
3.  From that folder, run `.\cabbie.exe service --install`

Cabbie service will now run as a service on that machine and check for updates
using the configuration options above.

## Enforcement Files

Cabbie enforcement files allow administrators to enforce specific update
policies on a per-device basis. Enforcement files use JSON format, and can be
written and maintained using any configuration management stack. Enforcements
are particularly useful when deploying Cabbie as a service.

To create an enforcement, place a json file (ending in .json) under the Cabbie
ProgramData directory (C:\ProgramData\Cabbie).

Required updates can be designated as a list of zero or more KB article strings
under the `required` key. To hide an update from cabbie, place the KB article
string under the `hidden` key.

Example:

```
{
  "required": [
    "123456"
  ],
  "hidden": [
    "456789"
  ]
}
```

## Using a Maintenance Window

You can define a maintenance window for Cabbie to follow by installing and
configuring the [aukera service](https://github.com/google/aukera). Once
configured, update the Cabbie registry options to `AukeraEnabled= 1` and restart
the Cabbie service.

### Example Aukera Config

```json
{
  "Windows": [
    {
      "Name": "Default Cabbie maintenance Window",
      "Format": 1,
      "Schedule": "0 40 10 * * THU",
      "Duration": "6h",
      "Starts": null,
      "Expires": null,
      "Labels": ["cabbie"]
    }
  ]
}
```

</details>

## Disclaimer

Cabbie is maintained by a small team at Google. Support for this repo is treated
as best effort, and issues will be responded to as engineering time permits.

This is not an official Google product.
