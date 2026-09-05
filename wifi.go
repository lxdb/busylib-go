package busylib

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
)

const (
	maxWiFiSSIDBytes     = 33
	maxWiFiPasswordBytes = 63
)

// WiFiService reads network state and controls Wi-Fi connections.
type WiFiService struct {
	client *Client
}

// WiFi returns the Wi-Fi control API.
func (c *Client) WiFi() WiFiService { return WiFiService{client: c} }

// WiFiStatus reports the current Wi-Fi connection and network settings.
type WiFiStatus struct {
	State    WiFiConnectionState `json:"state"`
	SSID     string              `json:"ssid,omitempty"`
	BSSID    string              `json:"bssid,omitempty"`
	Channel  int                 `json:"channel,omitempty"`
	RSSI     int                 `json:"rssi,omitempty"`
	Security WiFiSecurityMethod  `json:"security,omitempty"`
	IPConfig *WiFiIPConfig       `json:"ip_config,omitempty"`
}

// WiFiConnectionState identifies the Wi-Fi connection lifecycle state.
type WiFiConnectionState string

const (
	// WiFiStateUnknown means the firmware cannot determine the Wi-Fi state.
	WiFiStateUnknown WiFiConnectionState = "unknown"
	// WiFiStateDisconnected means no Wi-Fi network is connected.
	WiFiStateDisconnected WiFiConnectionState = "disconnected"
	// WiFiStateConnected means a Wi-Fi network is connected.
	WiFiStateConnected WiFiConnectionState = "connected"
	// WiFiStateConnecting means a connection attempt is in progress.
	WiFiStateConnecting WiFiConnectionState = "connecting"
	// WiFiStateDisconnecting means disconnection is in progress.
	WiFiStateDisconnecting WiFiConnectionState = "disconnecting"
	// WiFiStateReconnecting means a reconnection attempt is in progress.
	WiFiStateReconnecting WiFiConnectionState = "reconnecting"
)

// WiFiNetworkList contains the Wi-Fi networks found by a scan.
type WiFiNetworkList struct {
	Count    int           `json:"count"`
	Networks []WiFiNetwork `json:"networks"`
}

// WiFiNetwork describes one Wi-Fi network found by a scan.
type WiFiNetwork struct {
	SSID     string             `json:"ssid"`
	Security WiFiSecurityMethod `json:"security"`
	RSSI     int                `json:"rssi"`
}

// WiFiConnectRequest configures a Wi-Fi connection request. Password contains
// network credentials and should not be logged.
type WiFiConnectRequest struct {
	SSID     string              `json:"ssid"`
	Password string              `json:"password"`
	Security WiFiSecurityMethod  `json:"security"`
	IPConfig WiFiConnectIPConfig `json:"ip_config"`
}

// WiFiConnectIPConfig configures DHCP or a static IPv4 address.
type WiFiConnectIPConfig struct {
	IPMethod WiFiIPMethod `json:"ip_method"`
	Address  string       `json:"address,omitempty"`
	Mask     string       `json:"mask,omitempty"`
	Gateway  string       `json:"gateway,omitempty"`
}

// WiFiSecurityMethod identifies a firmware-supported Wi-Fi security mode.
type WiFiSecurityMethod string

const (
	// WiFiSecurityOpen selects an unsecured network.
	WiFiSecurityOpen WiFiSecurityMethod = "Open"
	// WiFiSecurityWPA selects WPA security.
	WiFiSecurityWPA WiFiSecurityMethod = "WPA"
	// WiFiSecurityWPA2 selects WPA2 security.
	WiFiSecurityWPA2 WiFiSecurityMethod = "WPA2"
	// WiFiSecurityWEP selects WEP security.
	WiFiSecurityWEP WiFiSecurityMethod = "WEP"
	// WiFiSecurityWPAWPA2 selects mixed WPA and WPA2 security.
	WiFiSecurityWPAWPA2 WiFiSecurityMethod = "WPA/WPA2"
	// WiFiSecurityWPA3 selects WPA3 security.
	WiFiSecurityWPA3 WiFiSecurityMethod = "WPA3"
	// WiFiSecurityWPA2WPA3 selects mixed WPA2 and WPA3 security.
	WiFiSecurityWPA2WPA3 WiFiSecurityMethod = "WPA2/WPA3"
	// WiFiSecurityUnsupported identifies an unsupported security mode.
	WiFiSecurityUnsupported WiFiSecurityMethod = "Unsupported"
)

// WiFiIPConfig reports the current Wi-Fi IP configuration.
type WiFiIPConfig struct {
	IPMethod WiFiIPMethod `json:"ip_method,omitempty"`
	IPType   WiFiIPType   `json:"ip_type,omitempty"`
	Address  string       `json:"address,omitempty"`
}

// WiFiIPMethod identifies dynamic or static address assignment.
type WiFiIPMethod string

const (
	// WiFiIPMethodDHCP requests dynamic address assignment.
	WiFiIPMethodDHCP WiFiIPMethod = "dhcp"
	// WiFiIPMethodStatic uses caller-provided address settings.
	WiFiIPMethodStatic WiFiIPMethod = "static"
)

// WiFiIPType identifies the address protocol version.
type WiFiIPType string

const (
	// WiFiIPTypeIPv4 identifies an IPv4 address.
	WiFiIPTypeIPv4 WiFiIPType = "ipv4"
	// WiFiIPTypeIPv6 identifies an IPv6 address.
	WiFiIPTypeIPv6 WiFiIPType = "ipv6"
)

// Status returns the current Wi-Fi connection details.
func (s WiFiService) Status(ctx context.Context) (WiFiStatus, error) {
	var out WiFiStatus
	err := s.client.doJSON(ctx, http.MethodGet, "/api/wifi/status", nil, nil, &out)
	return out, err
}

// Networks scans for available Wi-Fi networks. It is unavailable through
// remote.Client.Device.
func (s WiFiService) Networks(ctx context.Context) (WiFiNetworkList, error) {
	var out WiFiNetworkList
	err := s.client.doJSON(ctx, http.MethodGet, "/api/wifi/networks", nil, nil, &out)
	return out, err
}

// Connect validates the network settings and starts a Wi-Fi connection. It is
// unavailable through remote.Client.Device and can replace the caller's active
// network path.
func (s WiFiService) Connect(ctx context.Context, request WiFiConnectRequest) error {
	if err := request.Validate(); err != nil {
		return validationError(http.MethodPost, "/api/wifi/connect", err.Error(), err)
	}
	return s.client.doSuccess(ctx, http.MethodPost, "/api/wifi/connect", nil, JSONBody(request))
}

// Disconnect stops the current Wi-Fi connection. It is unavailable through
// remote.Client.Device and can remove the caller's active network path.
func (s WiFiService) Disconnect(ctx context.Context) error {
	return s.client.doSuccess(ctx, http.MethodPost, "/api/wifi/disconnect", nil, nil)
}

// Validate reports whether Wi-Fi connection settings meet the device contract.
func (request WiFiConnectRequest) Validate() error {
	if request.SSID == "" {
		return errors.New("ssid must not be empty")
	}
	if len(request.SSID) > maxWiFiSSIDBytes {
		return fmt.Errorf("ssid must be at most %d bytes", maxWiFiSSIDBytes)
	}
	if len(request.Password) > maxWiFiPasswordBytes {
		return fmt.Errorf("password must be at most %d bytes", maxWiFiPasswordBytes)
	}
	if !validWiFiConnectSecurity(request.Security) {
		return fmt.Errorf("security %q is not supported", request.Security)
	}
	config := request.IPConfig
	if !validWiFiIPMethod(config.IPMethod) {
		return fmt.Errorf("ip_config.ip_method %q is not supported", config.IPMethod)
	}
	if config.IPMethod == WiFiIPMethodStatic {
		if config.Address == "" || config.Mask == "" || config.Gateway == "" {
			return errors.New("static ip_config requires address, mask, and gateway")
		}
	}
	for field, value := range map[string]string{
		"address": config.Address,
		"mask":    config.Mask,
		"gateway": config.Gateway,
	} {
		if value == "" {
			continue
		}
		address, err := netip.ParseAddr(value)
		if err != nil || !address.Is4() {
			return fmt.Errorf("ip_config.%s must be an IPv4 address", field)
		}
	}
	return nil
}

func validWiFiConnectSecurity(value WiFiSecurityMethod) bool {
	switch value {
	case WiFiSecurityOpen, WiFiSecurityWPA, WiFiSecurityWPA2, WiFiSecurityWEP, WiFiSecurityWPAWPA2, WiFiSecurityWPA3, WiFiSecurityWPA2WPA3:
		return true
	default:
		return false
	}
}

func validWiFiIPMethod(value WiFiIPMethod) bool {
	switch value {
	case WiFiIPMethodDHCP, WiFiIPMethodStatic:
		return true
	default:
		return false
	}
}
