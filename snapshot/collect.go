package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	busylib "github.com/lxdb/busylib-go"
	"github.com/lxdb/busylib-go/proto/blepb"
	"github.com/lxdb/busylib-go/proto/statepb"
)

// Collect reads each firmware snapshot endpoint independently. Endpoint and
// payload failures stay on their fields; only invalid setup and caller
// cancellation are returned as operation errors.
func Collect(ctx context.Context, client *busylib.Client) (Snapshot, error) {
	var out Snapshot
	if ctx == nil {
		return out, errors.New("snapshot: context must not be nil")
	}
	if client == nil {
		return out, errors.New("snapshot: client must not be nil")
	}

	version, err := client.APISemVer(ctx)
	if err != nil {
		out.Version.Err = err
	} else {
		out.Version = present(version)
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}

	out.Name = collectJSON(ctx, client, "/api/name", decodeName)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Status = collectJSON(ctx, client, "/api/status", decodeValue[busylib.Status])
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.System = collectJSON(ctx, client, "/api/status/system", decodeValue[busylib.StatusSystem])
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Power = collectJSON(ctx, client, "/api/status/power", decodePower)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Time = collectJSON(ctx, client, "/api/time", decodeTime)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.WiFi = collectJSON(ctx, client, "/api/wifi/status", decodeWiFi)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Brightness = collectJSON(ctx, client, "/api/display/brightness", decodeBrightness)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.AudioVolume = collectJSON(ctx, client, "/api/audio/volume", decodeAudioVolume)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.BLE = collectJSON(ctx, client, "/api/ble/status", decodeBLE)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	out.Storage = collectJSON(ctx, client, "/api/storage/status", decodeValue[busylib.StorageStatus])
	if err := ctx.Err(); err != nil {
		return out, err
	}
	return out, nil
}

type decoder[T any] func([]byte) (T, error)

func collectJSON[T any](ctx context.Context, client *busylib.Client, path string, decode decoder[T]) Field[T] {
	response, err := client.Do(ctx, busylib.Request{
		Method:       http.MethodGet,
		Path:         path,
		ResponseMode: busylib.ResponseModeBytes,
	})
	if err != nil {
		return Field[T]{Err: err}
	}

	value, err := decode(response.Body)
	if err != nil {
		return Field[T]{
			Err: &busylib.ProtocolError{
				Method:    http.MethodGet,
				Path:      path,
				RequestID: response.RequestID,
				Excerpt:   protocolExcerpt(response.Body),
				Err:       err,
			},
			Raw: append([]byte(nil), response.Body...),
		}
	}
	return present(value)
}

func present[T any](value T) Field[T] {
	return Field[T]{Value: value, Present: true}
}

func decodeValue[T any](data []byte) (T, error) {
	var value T
	err := json.Unmarshal(data, &value)
	return value, err
}

func decodeName(data []byte) (string, error) {
	var payload struct {
		Name *string `json:"name"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if payload.Name == nil {
		return "", errors.New("response did not include name")
	}
	return *payload.Name, nil
}

func decodePower(data []byte) (Power, error) {
	var payload busylib.StatusPower
	if err := json.Unmarshal(data, &payload); err != nil {
		return Power{}, err
	}
	status, err := powerStatus(payload.State)
	if err != nil {
		return Power{}, err
	}
	charge, err := unsigned32("battery_charge", payload.BatteryCharge)
	if err != nil || charge > 100 {
		return Power{}, errors.New("battery_charge must be between 0 and 100")
	}
	voltage, err := unsigned32("battery_voltage", payload.BatteryVoltage)
	if err != nil {
		return Power{}, err
	}
	usbVoltage, err := unsigned32("usb_voltage", payload.USBVoltage)
	if err != nil {
		return Power{}, err
	}
	if payload.BatteryCurrent < math.MinInt32 || payload.BatteryCurrent > math.MaxInt32 {
		return Power{}, errors.New("battery_current does not fit signed 32 bits")
	}
	return Power{
		Known:                true,
		BatteryStatus:        status,
		BatteryChargePercent: charge,
		BatteryVoltageMV:     voltage,
		BatteryCurrentMA:     int32(payload.BatteryCurrent),
		USBVoltageMV:         usbVoltage,
	}, nil
}

func powerStatus(value busylib.PowerState) (statepb.BatteryStatus, error) {
	switch value {
	case busylib.PowerDischarging:
		return statepb.BatteryStatus_DISCHARGING, nil
	case busylib.PowerCharging:
		return statepb.BatteryStatus_CHARGING, nil
	case busylib.PowerCharged:
		return statepb.BatteryStatus_CHARGED, nil
	default:
		return 0, fmt.Errorf("unsupported power state %q", value)
	}
}

func unsigned32(field string, value int) (uint32, error) {
	if value < 0 || uint64(value) > math.MaxUint32 {
		return 0, fmt.Errorf("%s does not fit unsigned 32 bits", field)
	}
	return uint32(value), nil
}

func decodeTime(data []byte) (DeviceTime, error) {
	var payload struct {
		Timestamp *string `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return DeviceTime{}, err
	}
	if payload.Timestamp == nil {
		return DeviceTime{}, errors.New("response did not include timestamp")
	}
	value, err := time.Parse(time.RFC3339, *payload.Timestamp)
	if err != nil {
		return DeviceTime{}, fmt.Errorf("parse firmware timestamp: %w", err)
	}
	return DeviceTime{Timestamp: *payload.Timestamp, Time: value}, nil
}

func decodeBrightness(data []byte) (Brightness, error) {
	var payload struct {
		Value *string `json:"value"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return Brightness{}, err
	}
	if payload.Value == nil {
		return Brightness{}, errors.New("response did not include value")
	}
	if *payload.Value == "auto" {
		return Brightness{Mode: BrightnessModeAutomatic}, nil
	}
	value, err := strconv.ParseUint(*payload.Value, 10, 32)
	if err != nil || value > 100 {
		return Brightness{}, errors.New("brightness value must be auto or 0-100")
	}
	manual := uint32(value)
	return Brightness{Mode: BrightnessModeManual, Manual: &manual}, nil
}

func decodeAudioVolume(data []byte) (int, error) {
	var payload struct {
		Volume *int `json:"volume"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, err
	}
	if payload.Volume == nil || *payload.Volume < 0 || *payload.Volume > 100 {
		return 0, errors.New("volume must be between 0 and 100")
	}
	return *payload.Volume, nil
}

func decodeWiFi(data []byte) (WiFi, error) {
	var payload busylib.WiFiStatus
	if err := json.Unmarshal(data, &payload); err != nil {
		return WiFi{}, err
	}
	value := WiFi{
		SSID:     payload.SSID,
		BSSID:    payload.BSSID,
		RSSI:     int32(payload.RSSI),
		Security: payload.Security,
	}
	channel, err := unsigned32("channel", payload.Channel)
	if err != nil {
		return WiFi{}, err
	}
	value.Channel = channel
	value.State, value.ConnectionStatus, err = wifiState(payload.State)
	if err != nil {
		return WiFi{}, err
	}
	if payload.Security != "" {
		security := wifiSecurity(payload.Security)
		value.SecurityCode = &security
	}
	if payload.IPConfig != nil {
		address, err := wifiIPAddress(*payload.IPConfig)
		if err != nil {
			return WiFi{}, err
		}
		value.IPAddresses = []IPAddress{address}
	}
	return value, nil
}

func wifiState(value busylib.WiFiConnectionState) (WiFiState, *statepb.WifiConnectionStatus, error) {
	var status statepb.WifiConnectionStatus
	switch value {
	case busylib.WiFiStateUnknown:
		return WiFiStateUnknown, nil, nil
	case busylib.WiFiStateDisconnected:
		return WiFiStateInactive, nil, nil
	case busylib.WiFiStateConnected:
		status = statepb.WifiConnectionStatus_CONNECTED
	case busylib.WiFiStateConnecting:
		status = statepb.WifiConnectionStatus_CONNECTING
	case busylib.WiFiStateDisconnecting:
		status = statepb.WifiConnectionStatus_DISCONNECTING
	case busylib.WiFiStateReconnecting:
		status = statepb.WifiConnectionStatus_RECONNECTING
	default:
		return "", nil, fmt.Errorf("unsupported Wi-Fi state %q", value)
	}
	return WiFiStateActive, &status, nil
}

func wifiSecurity(value busylib.WiFiSecurityMethod) statepb.WifiSecurity {
	switch value {
	case busylib.WiFiSecurityOpen:
		return statepb.WifiSecurity_OPEN
	case busylib.WiFiSecurityWPA:
		return statepb.WifiSecurity_WPA
	case busylib.WiFiSecurityWPA2:
		return statepb.WifiSecurity_WPA2
	case busylib.WiFiSecurityWEP:
		return statepb.WifiSecurity_WEP
	case busylib.WiFiSecurityWPAWPA2:
		return statepb.WifiSecurity_WPA_WPA2
	case busylib.WiFiSecurityWPA3:
		return statepb.WifiSecurity_WPA3
	case busylib.WiFiSecurityWPA2WPA3:
		return statepb.WifiSecurity_WPA2_WPA3
	default:
		return statepb.WifiSecurity_UNKNOWN
	}
}

func wifiIPAddress(value busylib.WiFiIPConfig) (IPAddress, error) {
	var out IPAddress
	switch value.IPType {
	case busylib.WiFiIPTypeIPv4:
		out.Protocol = statepb.IpProtocol_IPV4
	case busylib.WiFiIPTypeIPv6:
		out.Protocol = statepb.IpProtocol_IPV6
	default:
		return out, fmt.Errorf("unsupported Wi-Fi IP type %q", value.IPType)
	}
	switch value.IPMethod {
	case busylib.WiFiIPMethodDHCP:
		out.Method = statepb.IpConfigurationMethod_DHCP
	case busylib.WiFiIPMethodStatic:
		out.Method = statepb.IpConfigurationMethod_STATIC
	default:
		return out, fmt.Errorf("unsupported Wi-Fi IP method %q", value.IPMethod)
	}
	out.Address = value.Address
	return out, nil
}

func decodeBLE(data []byte) (BLE, error) {
	var payload busylib.BleStatusResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return BLE{}, err
	}
	status, err := bleStatus(payload.Status)
	if err != nil {
		return BLE{}, err
	}
	value := BLE{Status: status}
	if payload.Address != "" {
		address := payload.Address
		value.Address = &address
	}
	if status == blepb.ServiceStatus_CONNECTED && value.Address == nil {
		return BLE{}, errors.New("connected BLE response did not include address")
	}
	return value, nil
}

func bleStatus(value busylib.BLEStatus) (blepb.ServiceStatus, error) {
	switch value {
	case busylib.BLEStatusReset:
		return blepb.ServiceStatus_RESET, nil
	case busylib.BLEStatusInitialization:
		return blepb.ServiceStatus_INITIALIZATION, nil
	case busylib.BLEStatusDisabled:
		return blepb.ServiceStatus_READY, nil
	case busylib.BLEStatusEnabled:
		return blepb.ServiceStatus_ADVERTISING, nil
	case busylib.BLEStatusConnectable:
		return blepb.ServiceStatus_CONNECTABLE, nil
	case busylib.BLEStatusConnected:
		return blepb.ServiceStatus_CONNECTED, nil
	default:
		return 0, fmt.Errorf("unsupported BLE status %q", value)
	}
}

func protocolExcerpt(data []byte) string {
	const limit = 256
	if len(data) <= limit {
		return string(data)
	}
	return string(data[:limit]) + "..."
}
