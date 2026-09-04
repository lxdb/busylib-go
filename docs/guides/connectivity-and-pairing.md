# Connectivity and pairing

Inspect current connection state before changing Wi-Fi, Bluetooth Low Energy, account, or smart-home settings. Some operations can disconnect the current control path.

## Inspect Wi-Fi

```go
status, err := client.WiFi().Status(ctx)
if err != nil {
	return err
}
log.Printf("Wi-Fi state: %s", status.State)

networks, err := client.WiFi().Networks(ctx)
if err != nil {
	return err
}
log.Printf("visible networks: %d", len(networks.Networks))
```

`Networks`, `Connect`, and `Disconnect` are local-only because the firmware MQTT proxy blocks operations that can change remote reachability.

## Connect to Wi-Fi

```go
request := busylib.WiFiConnectRequest{
	SSID:     ssid,
	Password: password,
	Security: busylib.WiFiSecurityWPA2,
	IPConfig: busylib.WiFiConnectIPConfig{
		IPMethod: busylib.WiFiIPMethodDHCP,
	},
}
if err := client.WiFi().Connect(ctx, request); err != nil {
	return err
}
```

Do not log the password. A successful request means the device accepted the connection request; read `WiFi().Status` to observe the resulting state.

`Disconnect` can remove the network path used by the caller. Use a local connection path that can survive or recover from that change.

## Control Bluetooth Low Energy

```go
status, err := client.BLE().Status(ctx)
if err != nil {
	return err
}
if status.State == busylib.BLEStateDisabled {
	if err := client.BLE().Enable(ctx); err != nil {
		return err
	}
}
```

The `enabled`, `connectable`, and `connected` states all mean that BLE is active. Do not call `Enable` again for those states. Wait while BLE is resetting or initializing, and handle `BLEStateInternalError` as a device failure.

`Disable` turns off BLE support. `RemovePairing` removes the saved pairing and requires clients to pair again.

## Link a remote account

```go
status, err := client.Account().Status(ctx)
if err != nil {
	return err
}
log.Printf("account state: %s", status.Status)

link, err := client.Account().Link(ctx)
if err != nil {
	return err
}
log.Printf("account link code: %s (expires at %d)", link.Code, link.ExpiresAt)
```

`Link`, `SetBackend`, and `Unlink` are local-only through the firmware MQTT proxy. `Unlink` disconnects the device from the current account. Do not call it as cleanup for a read-only workflow.

Account authorization values can be sensitive. Log only the information the application intentionally presents to its user.

## Configure smart-home behavior

```go
pairing, err := client.SmartHome().PairingStatus(ctx)
if err != nil {
	return err
}
log.Printf("pairing state: %s", pairing.LatestPairingStatus.Value)

setup, err := client.SmartHome().StartPairing(ctx)
if err != nil {
	return err
}
log.Printf("pairing code available: %t", setup.ManualCode != "")
```

`ForgetPairings` removes all saved smart-home pairings. `SetSwitchState` can change the current switch state, startup behavior, or both. Read `SwitchState` first when the application must preserve a setting.

The [service reference](../reference/services.md#remote-mqtt-restrictions) lists every local-only method and its firmware operation.
