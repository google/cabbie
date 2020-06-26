# Cabbie

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

| Setting           | Registry Type| Default Setting   | Description                                                                                              |
|-------------------|--------------|-------------------|----------------------------------------------------------------------------------------------------------|
| ------------------| -------------| ------------------| ------------------                                                                                       |
| WSUSServers       |REG_MULTI_SZ  |nil                |List of WSUS servers to connect to instead of Microsoft updates.                                          |
| RequiredCategories|REG_MULTI_SZ  |"Critical          |List of Update categories that an update must contain at least one of to be automatically installed.      |
:                   :              : Updates",         :                                                                                                          :
:                   :              : "Definition       :                                                                                                          :
:                   :              : Updates",         :                                                                                                          :
:                   :              : "Security Updates":                                                                                                          :
| UpdateDrivers     |REG_DWORD     |0                  |Allow Cabbie to install available drivers.                                                                |
:                   :              :                   :                                                                                                          :
:                   :              :                   :0 = Disabled                                                                                              :
:                   :              :                   :1 = Enabled                                                                                               :
| UpdateVirusDef    |REG_DWORD     |1                  |Allow Cabbie to install updated virus definitions every 30 minutes.                                       |
:                   :              :                   :                                                                                                          :
:                   :              :                   :0 = Disabled                                                                                              :
:                   :              :                   :1 = Enabled                                                                                               :
| EnableThirdParty  |REG_DWORD     |0                  |Allow Cabbie to check for third party software updates such off MSFT Office and Adobe.                    |
:                   :              :                   :                                                                                                          :
:                   :              :                   :0 = Disabled                                                                                              :
:                   :              :                   :1 = Enabled                                                                                               :
| RebootDelay       |REG_DWORD     |21600              |Time in seconds for Cabbie to wait before force rebooting a machine to finalize update installation.      |
| Deadline          |REG_DWORD     |14                 |Number of days before Cabbie will force install an available update that matches the required categories. |
:                   :              :                   :                                                                                                          :
:                   :              :                   :Set to "0" to disable this option.                                                                        :
| AukeraEnabled     |REG_DWORD     |0                  |Enable Cabbie to use the open source Aukera maintenance window manager.                                   |
:                   :              :                   :                                                                                                          :
:                   :              :                   :0 = Disabled                                                                                              :
:                   :              :                   :1 = Enabled                                                                                               :
| AukeraPort        |REG_DWORD     |9119               |LocalHost port to check against for Aukera maintenance windows.                                           |
| AukeraName         |REG_SZ        |"Cabbie"           |Aukera maintenance window label to query for to determine if a maintenance window is currently open.      |
| NotifyAvailable    |REG_DWORD     |1                  |If enabled Cabbie will send a notification when new required updates are available to be installed.       |



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

`cabbie install --drivers`


Update virus definitions:

`cabbie install --virus_def`


Install specific update KBs:

`cabbie install --kbs="1234513,98765432"`


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

Cabbie can also run as a Windows Service to enable constant update and reboot management.

The basic steps to install the Cabbie service:

1.   Compile the Cabbie binary using `go build C:\Path\to\cabbie\src`.
2.   Copy the built `cabbie.exe` binary to the folder `C:\Program Files\Google\Cabbie`.
3.   From that folder, run `.\cabbie.exe service --install`

Cabbie service will now run as a service on that machine and check for updates using the configuration options above.

### Using a Maintenance Window

You can define a maintenance window for Cabbie to follow by installing and configuring the [aukera service](https://github.com/google/aukera). Once configured, update the Cabbie registry options to `AukeraEnabled= 1` and restart the Cabbie service.

<details>
  <summary>Example Aukera Config</summary>
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

Cabbie is maintained by a small team at Google. Support for this repo is
treated as best effort, and issues will be responded to as engineering time
permits.

This is not an official Google product.
