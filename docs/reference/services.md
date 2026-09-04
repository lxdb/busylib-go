# Service reference

`busylib.Client` groups the BUSY Bar HTTP API into 14 services. Create or obtain one client, choose the service that owns the task, and pass a bounded context to each method.

The service types, requests, and responses do not change with the transport:

| Client entry point | Route to the device | Service availability |
| --- | --- | --- |
| `busylib.NewClient()` | Local HTTP | All root services |
| `remote.Client.Device()` | Firmware MQTT HTTP proxy | All root services except the seven operations under [Remote MQTT restrictions](#remote-mqtt-restrictions) |
| `ble.Client.Device()` | Raw HTTP over the Nordic UART Service | All root services, subject to the [BLE HTTP transport contract](../../ble/README.md#http-transport-contract) |

`ble.Client.Device()` returns the same `*busylib.Client` used for local HTTP. The BLE module changes the HTTP transport; it does not duplicate services or translate individual service methods into BLE-specific commands.

## System

Use `client.System()` to read identity, health, transport, power, runtime, and diagnostic information.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.System().Version`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.Version) | Read firmware and API version details. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.System().Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.Status) | Read the aggregate device status. | Yes | [Getting started](../getting-started.md) |
| [`client.System().DeviceStatus`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.DeviceStatus) | Read device identity and hardware status. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.System().FirmwareStatus`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.FirmwareStatus) | Read firmware build and version status. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.System().SystemStatus`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.SystemStatus) | Read runtime and memory status. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.System().PowerStatus`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.PowerStatus) | Read battery and power-source status. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.System().Transport`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.Transport) | Read the network interface used by the API. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.System().DumpLog`](https://pkg.go.dev/github.com/lxdb/busylib-go#SystemService.DumpLog) | Create a device log archive and return its storage path. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |

## Settings

Use `client.Settings()` to read or change device-wide HTTP access and naming settings.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Settings().HTTPAccess`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.HTTPAccess) | Read the local HTTP access mode. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Settings().SetHTTPAccess`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.SetHTTPAccess) | Change the local HTTP access mode and optional key. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Settings().AccessTokens`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.AccessTokens) | List stored access-token metadata without secrets. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Settings().MintAccessToken`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.MintAccessToken) | Create an access token and return its one-time secret. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Settings().RevokeAccessToken`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.RevokeAccessToken) | Remove one access token by short ID. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Settings().RevokeAllAccessTokens`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.RevokeAllAccessTokens) | Remove every stored access token. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Settings().Name`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.Name) | Read the device name. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Settings().SetName`](https://pkg.go.dev/github.com/lxdb/busylib-go#SettingsService.SetName) | Validate and change the device name. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |

## Display

Use `client.Display()` to control brightness, draw application elements, clear content, and capture a physical display.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Display().Brightness`](https://pkg.go.dev/github.com/lxdb/busylib-go#DisplayService.Brightness) | Read the brightness value or automatic mode. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Display().SetBrightness`](https://pkg.go.dev/github.com/lxdb/busylib-go#DisplayService.SetBrightness) | Select automatic brightness or a percentage. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Display().Draw`](https://pkg.go.dev/github.com/lxdb/busylib-go#DisplayService.Draw) | Validate and render application elements. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Display().Clear`](https://pkg.go.dev/github.com/lxdb/busylib-go#DisplayService.Clear) | Remove one application's elements or all rendered elements. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Display().ClearElements`](https://pkg.go.dev/github.com/lxdb/busylib-go#DisplayService.ClearElements) | Remove selected element IDs from one application or all applications. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Display().Screen`](https://pkg.go.dev/github.com/lxdb/busylib-go#DisplayService.Screen) | Capture decoded pixel bytes from the selected display. | Yes | [Display and media](../guides/display-and-media.md) |

## Audio

Use `client.Audio()` to start and stop playback or change output volume.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Audio().Play`](https://pkg.go.dev/github.com/lxdb/busylib-go#AudioService.Play) | Validate and play an uploaded or stock audio asset. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Audio().PlayAsset`](https://pkg.go.dev/github.com/lxdb/busylib-go#AudioService.PlayAsset) | Play an uploaded application asset. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Audio().PlayStock`](https://pkg.go.dev/github.com/lxdb/busylib-go#AudioService.PlayStock) | Play a firmware stock asset. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Audio().Stop`](https://pkg.go.dev/github.com/lxdb/busylib-go#AudioService.Stop) | Stop current playback. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Audio().Volume`](https://pkg.go.dev/github.com/lxdb/busylib-go#AudioService.Volume) | Read the output volume. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Audio().SetVolume`](https://pkg.go.dev/github.com/lxdb/busylib-go#AudioService.SetVolume) | Change output volume and select device feedback behavior. | Yes | [Display and media](../guides/display-and-media.md) |
| [`client.Audio().SetVolumeSilently`](https://pkg.go.dev/github.com/lxdb/busylib-go#AudioService.SetVolumeSilently) | Change output volume without device feedback. | Yes | [Display and media](../guides/display-and-media.md) |

## Assets

Use `client.Assets()` to upload and remove files owned by one application.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Assets().Upload`](https://pkg.go.dev/github.com/lxdb/busylib-go#AssetsService.Upload) | Upload an application asset from a `busylib.Body`. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Assets().UploadFile`](https://pkg.go.dev/github.com/lxdb/busylib-go#AssetsService.UploadFile) | Upload a local file as an application asset. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Assets().DeleteApplicationAssets`](https://pkg.go.dev/github.com/lxdb/busylib-go#AssetsService.DeleteApplicationAssets) | Remove all assets owned by one application. | Yes | [Assets and storage](../guides/assets-and-storage.md) |

## Storage

Use `client.Storage()` to manage files and directories in device storage.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Storage().Write`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.Write) | Replace or append to a device file from a `busylib.Body`. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().WriteFile`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.WriteFile) | Upload a local file to a device path. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().Read`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.Read) | Read a bounded device file into memory. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().ReadTo`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.ReadTo) | Stream a device file to an `io.Writer`. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().List`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.List) | List entries below a device directory. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().Remove`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.Remove) | Delete a device file or directory. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().Mkdir`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.Mkdir) | Create a device directory. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().Rename`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.Rename) | Move or rename a device file or directory. | Yes | [Assets and storage](../guides/assets-and-storage.md) |
| [`client.Storage().Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#StorageService.Status) | Read storage capacity and usage. | Yes | [Assets and storage](../guides/assets-and-storage.md) |

## Busy state

Use `client.Busy()` to read or replace the active busy snapshot and saved profiles.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Busy().Snapshot`](https://pkg.go.dev/github.com/lxdb/busylib-go#BusyService.Snapshot) | Read the current busy-state snapshot. | Yes | [Busy state](../guides/busy-state.md) |
| [`client.Busy().SetSnapshot`](https://pkg.go.dev/github.com/lxdb/busylib-go#BusyService.SetSnapshot) | Validate and replace the current busy-state snapshot. | Yes | [Busy state](../guides/busy-state.md) |
| [`client.Busy().Profile`](https://pkg.go.dev/github.com/lxdb/busylib-go#BusyService.Profile) | Read a saved busy profile by slot. | Yes | [Busy state](../guides/busy-state.md) |
| [`client.Busy().SetProfile`](https://pkg.go.dev/github.com/lxdb/busylib-go#BusyService.SetProfile) | Validate and replace a saved busy profile. | Yes | [Busy state](../guides/busy-state.md) |

## Account

Use `client.Account()` to inspect or change the device's remote account link and backend.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Account().Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#AccountService.Status) | Read the account connection state. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.Account().Info`](https://pkg.go.dev/github.com/lxdb/busylib-go#AccountService.Info) | Read linked account details. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.Account().Link`](https://pkg.go.dev/github.com/lxdb/busylib-go#AccountService.Link) | Start account linking and return authorization details. | No; local only | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.Account().Backend`](https://pkg.go.dev/github.com/lxdb/busylib-go#AccountService.Backend) | Read the account backend configuration. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.Account().SetBackend`](https://pkg.go.dev/github.com/lxdb/busylib-go#AccountService.SetBackend) | Validate and replace the account backend configuration. | No; local only | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.Account().Unlink`](https://pkg.go.dev/github.com/lxdb/busylib-go#AccountService.Unlink) | Disconnect the device from its remote account. | No; local only | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |

## Bluetooth Low Energy

Use `client.BLE()` to inspect Bluetooth Low Energy state and control pairing availability.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.BLE().Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#BLEService.Status) | Read BLE state and pairing details. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.BLE().Enable`](https://pkg.go.dev/github.com/lxdb/busylib-go#BLEService.Enable) | Enable BLE support. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.BLE().Disable`](https://pkg.go.dev/github.com/lxdb/busylib-go#BLEService.Disable) | Disable BLE support. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.BLE().RemovePairing`](https://pkg.go.dev/github.com/lxdb/busylib-go#BLEService.RemovePairing) | Remove the saved BLE pairing. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |

## Wi-Fi

Use `client.WiFi()` to inspect network state, scan, connect, or disconnect.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.WiFi().Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#WiFiService.Status) | Read current Wi-Fi connection details. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.WiFi().Networks`](https://pkg.go.dev/github.com/lxdb/busylib-go#WiFiService.Networks) | Scan for available Wi-Fi networks. | No; local only | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.WiFi().Connect`](https://pkg.go.dev/github.com/lxdb/busylib-go#WiFiService.Connect) | Validate settings and start a Wi-Fi connection. | No; local only | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.WiFi().Disconnect`](https://pkg.go.dev/github.com/lxdb/busylib-go#WiFiService.Disconnect) | Stop the current Wi-Fi connection. | No; local only | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |

## Input

Use `client.Input()` to send a virtual key press.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Input().SendKey`](https://pkg.go.dev/github.com/lxdb/busylib-go#InputService.SendKey) | Validate and send one virtual key press. | Yes | [Display and media](../guides/display-and-media.md) |

## Smart home

Use `client.SmartHome()` to control pairing and switch behavior.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.SmartHome().PairingStatus`](https://pkg.go.dev/github.com/lxdb/busylib-go#SmartHomeService.PairingStatus) | Read the smart-home pairing state. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.SmartHome().StartPairing`](https://pkg.go.dev/github.com/lxdb/busylib-go#SmartHomeService.StartPairing) | Start pairing and return setup data. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.SmartHome().ForgetPairings`](https://pkg.go.dev/github.com/lxdb/busylib-go#SmartHomeService.ForgetPairings) | Remove all saved smart-home pairings. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.SmartHome().SwitchState`](https://pkg.go.dev/github.com/lxdb/busylib-go#SmartHomeService.SwitchState) | Read the switch configuration. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |
| [`client.SmartHome().SetSwitchState`](https://pkg.go.dev/github.com/lxdb/busylib-go#SmartHomeService.SetSwitchState) | Validate and change the switch configuration. | Yes | [Connectivity and pairing](../guides/connectivity-and-pairing.md) |

## Time

Use `client.Time()` to read or change the device clock and timezone.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Time().Now`](https://pkg.go.dev/github.com/lxdb/busylib-go#TimeService.Now) | Read the device timestamp. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Time().SetTimestamp`](https://pkg.go.dev/github.com/lxdb/busylib-go#TimeService.SetTimestamp) | Set the clock from an RFC 3339 timestamp. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Time().Timezone`](https://pkg.go.dev/github.com/lxdb/busylib-go#TimeService.Timezone) | Read the current timezone. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Time().SetTimezone`](https://pkg.go.dev/github.com/lxdb/busylib-go#TimeService.SetTimezone) | Change the device timezone. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |
| [`client.Time().Timezones`](https://pkg.go.dev/github.com/lxdb/busylib-go#TimeService.Timezones) | List timezone names supported by the device. | Yes | [Inspect and configure](../guides/inspect-and-configure.md) |

## Update

Use `client.Update()` to check, upload, install, stop, and configure firmware updates.

| Method | Effect | Remote MQTT | Guide |
| --- | --- | --- | --- |
| [`client.Update().UploadPackage`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.UploadPackage) | Upload a firmware package from a `busylib.Body`. | No; local only | [Firmware updates](../guides/firmware-updates.md) |
| [`client.Update().Check`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.Check) | Ask the device to check for available firmware. | Yes | [Firmware updates](../guides/firmware-updates.md) |
| [`client.Update().Status`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.Status) | Read current update state. | Yes | [Firmware updates](../guides/firmware-updates.md) |
| [`client.Update().Changelog`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.Changelog) | Read release notes for a firmware version. | Yes | [Firmware updates](../guides/firmware-updates.md) |
| [`client.Update().Install`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.Install) | Start downloading and installing a firmware version. | Yes | [Firmware updates](../guides/firmware-updates.md) |
| [`client.Update().AbortDownload`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.AbortDownload) | Stop the active firmware download. | Yes | [Firmware updates](../guides/firmware-updates.md) |
| [`client.Update().AutoUpdate`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.AutoUpdate) | Read automatic-update settings. | Yes | [Firmware updates](../guides/firmware-updates.md) |
| [`client.Update().SetAutoUpdate`](https://pkg.go.dev/github.com/lxdb/busylib-go#UpdateService.SetAutoUpdate) | Validate and apply automatic-update settings. | Yes | [Firmware updates](../guides/firmware-updates.md) |

## Remote MQTT restrictions

The firmware MQTT HTTP proxy blocks these operations. `remote.Client.Device()` rejects them before it publishes a request.

| Go method | Firmware operation |
| --- | --- |
| `client.Update().UploadPackage` | `POST /api/update` |
| `client.Account().Unlink` | `DELETE /api/account` |
| `client.Account().Link` | `POST /api/account/link` |
| `client.Account().SetBackend` | `PUT /api/account/backend` |
| `client.WiFi().Connect` | `POST /api/wifi/connect` |
| `client.WiFi().Disconnect` | `POST /api/wifi/disconnect` |
| `client.WiFi().Networks` | `GET /api/wifi/networks` |

Read [Remote MQTT](../integrations/remote-mqtt.md) for connection ownership and shutdown order.
