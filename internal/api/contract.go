package api

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
)

const (
	// ExpectedAPIVersion is the audited firmware API version.
	ExpectedAPIVersion = "27.5.0"
	// ExpectedOperationCount is the total number of audited HTTP operations.
	ExpectedOperationCount = 72
	// ExpectedSyncOperationCount is the number of synchronous HTTP operations.
	ExpectedSyncOperationCount = 71
	// StreamPhase is the firmware owner phase for the status stream.
	StreamPhase = 6
	// ExpectedStreamUpdateKinds is the audited number of typed stream updates.
	ExpectedStreamUpdateKinds = 15
)

// Contract is an independently recorded audit of the BUSY Bar firmware HTTP
// handlers. It contains contract facts and source provenance, not copied
// firmware implementation code.
type Contract struct {
	Repository     string                `json:"repository"`
	Branch         string                `json:"branch"`
	FirmwareCommit string                `json:"firmwareCommit"`
	APIVersion     string                `json:"apiVersion"`
	ProtobufCommit string                `json:"protobufCommit"`
	License        string                `json:"license"`
	StatusStream   StatusStreamContract  `json:"statusStream"`
	Frames         FrameContract         `json:"frames"`
	Snapshots      SnapshotContract      `json:"snapshots"`
	OptionalTools  OptionalToolsContract `json:"optionalTools"`
	Remote         RemoteContract        `json:"remote"`
	Operations     []Operation           `json:"operations"`
}

// StatusStreamContract records the audited WebSocket stream protocol.
type StatusStreamContract struct {
	Path                      string            `json:"path"`
	AccessKeyQuery            string            `json:"accessKeyQuery"`
	APISemVerQuery            string            `json:"apiSemVerQuery"`
	InitialControl            string            `json:"initialControl"`
	SnapshotControl           string            `json:"snapshotControl"`
	MaxClients                int               `json:"maxClients"`
	FrameIntervalMS           int               `json:"frameIntervalMs"`
	PublisherHeartbeatMS      int               `json:"publisherHeartbeatMs"`
	ClientHeartbeatIntervalMS int               `json:"clientHeartbeatIntervalMs"`
	RateLimitMaxPackets       int               `json:"rateLimitMaxPackets"`
	RateLimitPeriodMS         int               `json:"rateLimitPeriodMs"`
	StateUpdateKinds          int               `json:"stateUpdateKinds"`
	FrontFramesOnly           bool              `json:"frontFramesOnly"`
	FatalErrorCause           string            `json:"fatalErrorCause"`
	SourceReferences          []SourceReference `json:"sourceReferences"`
}

// FrameContract records the audited HTTP and protobuf frame formats.
type FrameContract struct {
	HTTPPath             string               `json:"httpPath"`
	HTTPEncoding         string               `json:"httpEncoding"`
	MaxPayloadBytes      int                  `json:"maxPayloadBytes"`
	EmittedEncodings     []string             `json:"emittedEncodings"`
	ProtobufPixelFormats []string             `json:"protobufPixelFormats"`
	Front                FrameSurfaceContract `json:"front"`
	Back                 FrameSurfaceContract `json:"back"`
	SourceReferences     []SourceReference    `json:"sourceReferences"`
}

// FrameSurfaceContract records one physical display's pixel layout.
type FrameSurfaceContract struct {
	Screen        int    `json:"screen"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	PixelFormat   string `json:"pixelFormat"`
	PlainBytes    int    `json:"plainBytes"`
	WireLayout    string `json:"wireLayout"`
	RLEBlockBytes int    `json:"rleBlockBytes"`
}

// SnapshotContract records the HTTP and stream sources used by snapshots.
type SnapshotContract struct {
	HTTP             []SnapshotHTTPContract `json:"http"`
	StateUpdateKinds []string               `json:"stateUpdateKinds"`
	SourceReferences []SourceReference      `json:"sourceReferences"`
}

// SnapshotHTTPContract records one independently collected snapshot section.
type SnapshotHTTPContract struct {
	Section       string   `json:"section"`
	Path          string   `json:"path"`
	CanonicalKeys []string `json:"canonicalKeys"`
}

// OptionalToolsContract records the firmware contracts behind optional tools.
type OptionalToolsContract struct {
	CLI   CLIContract   `json:"cli"`
	Media MediaContract `json:"media"`
}

// CLIContract records the audited USB-network CLI protocol.
type CLIContract struct {
	DefaultAddress   string               `json:"defaultAddress"`
	Port             int                  `json:"port"`
	Prompt           string               `json:"prompt"`
	InterruptByte    int                  `json:"interruptByte"`
	RebootCommand    string               `json:"rebootCommand"`
	Commands         []CLICommandContract `json:"commands"`
	SourceReferences []SourceReference    `json:"sourceReferences"`
}

// CLICommandContract records one curated firmware CLI command.
type CLICommandContract struct {
	Name         string `json:"name"`
	Mode         string `json:"mode"`
	SourceFile   string `json:"sourceFile"`
	SourceSymbol string `json:"sourceSymbol"`
}

// MediaContract records the firmware-compatible media formats.
type MediaContract struct {
	Image            ImageConversionContract `json:"image"`
	Audio            AudioConversionContract `json:"audio"`
	SourceReferences []SourceReference       `json:"sourceReferences"`
}

// ImageConversionContract records supported image output dimensions and format.
type ImageConversionContract struct {
	OutputFormat   string `json:"outputFormat"`
	Decoder        string `json:"decoder"`
	FrontMaxWidth  int    `json:"frontMaxWidth"`
	FrontMaxHeight int    `json:"frontMaxHeight"`
	BackMaxWidth   int    `json:"backMaxWidth"`
	BackMaxHeight  int    `json:"backMaxHeight"`
}

// AudioConversionContract records the required device audio format.
type AudioConversionContract struct {
	Header          string `json:"header"`
	Channels        int    `json:"channels"`
	SampleRateHz    int    `json:"sampleRateHz"`
	BitsPerSample   int    `json:"bitsPerSample"`
	ByteOrder       string `json:"byteOrder"`
	OutputExtension string `json:"outputExtension"`
}

// RemoteContract records the audited MQTT HTTP and stream protocols.
type RemoteContract struct {
	MQTTVersion      int                  `json:"mqttVersion"`
	APIVersion       string               `json:"apiVersion"`
	TopicPattern     string               `json:"topicPattern"`
	DownDirection    string               `json:"downDirection"`
	UpDirection      string               `json:"upDirection"`
	HTTP             RemoteHTTPContract   `json:"http"`
	Stream           RemoteStreamContract `json:"stream"`
	SourceReferences []SourceReference    `json:"sourceReferences"`
}

// RemoteHTTPContract records the MQTT HTTP request-response protocol.
type RemoteHTTPContract struct {
	RequestTopic            string   `json:"requestTopic"`
	LocalHost               string   `json:"localHost"`
	PathPrefix              string   `json:"pathPrefix"`
	TimeoutMS               int      `json:"timeoutMs"`
	RequestQoS              int      `json:"requestQos"`
	ResponseQoS             int      `json:"responseQos"`
	InvalidStatus           int      `json:"invalidStatus"`
	RequiresResponseTopic   bool     `json:"requiresResponseTopic"`
	RequiresCorrelationData bool     `json:"requiresCorrelationData"`
	EchoesCorrelationData   bool     `json:"echoesCorrelationData"`
	BlockedOperations       []string `json:"blockedOperations"`
}

// RemoteStreamContract records the MQTT status-stream protocol.
type RemoteStreamContract struct {
	RequestTopic                   string `json:"requestTopic"`
	RequestQoS                     int    `json:"requestQos"`
	ResponseQoS                    int    `json:"responseQos"`
	DefaultExpirySeconds           int    `json:"defaultExpirySeconds"`
	FrameIntervalMS                int    `json:"frameIntervalMs"`
	QueueSize                      int    `json:"queueSize"`
	EmptyPayloadStops              bool   `json:"emptyPayloadStops"`
	NonEmptyPayloadStarts          bool   `json:"nonEmptyPayloadStarts"`
	SnapshotOnStart                bool   `json:"snapshotOnStart"`
	SinglePublisher                bool   `json:"singlePublisher"`
	MessageLimitMaxCountKey        string `json:"messageLimitMaxCountKey"`
	MessageLimitIntervalSecondsKey string `json:"messageLimitIntervalSecondsKey"`
}

// SourceReference identifies the firmware source behind one contract fact.
type SourceReference struct {
	SourceFile   string `json:"sourceFile"`
	SourceSymbol string `json:"sourceSymbol"`
}

// Operation records one firmware HTTP method, path, owner, and source location.
type Operation struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Phase        int    `json:"phase"`
	SourceFile   string `json:"sourceFile"`
	SourceSymbol string `json:"sourceSymbol"`
}

// ID returns the operation as an uppercase method followed by its path.
func (o Operation) ID() string {
	return strings.ToUpper(o.Method) + " " + o.Path
}

// LoadContractFile reads, decodes, and validates a recorded contract.
func LoadContractFile(path string) (Contract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, err
	}

	var contract Contract
	if err := json.Unmarshal(data, &contract); err != nil {
		return Contract{}, err
	}
	if err := contract.Validate(); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

// Validate checks the complete recorded contract against audited invariants.
func (c Contract) Validate() error {
	if c.Repository == "" || c.Branch == "" || c.FirmwareCommit == "" || c.ProtobufCommit == "" {
		return fmt.Errorf("firmware provenance is incomplete")
	}
	if c.APIVersion != ExpectedAPIVersion {
		return fmt.Errorf("firmware API version = %q, want %q", c.APIVersion, ExpectedAPIVersion)
	}
	if len(c.Operations) != ExpectedOperationCount {
		return fmt.Errorf("operation count = %d, want %d", len(c.Operations), ExpectedOperationCount)
	}
	if err := c.StatusStream.Validate(); err != nil {
		return err
	}
	if err := c.Frames.Validate(); err != nil {
		return err
	}
	if err := c.Snapshots.Validate(); err != nil {
		return err
	}
	if err := c.OptionalTools.Validate(); err != nil {
		return err
	}
	if err := c.Remote.Validate(); err != nil {
		return err
	}

	seen := make(map[string]struct{}, len(c.Operations))
	syncCount := 0
	streamCount := 0
	for _, operation := range c.Operations {
		id := operation.ID()
		if operation.Method == "" || operation.Path == "" || operation.SourceFile == "" || operation.SourceSymbol == "" {
			return fmt.Errorf("operation %q has incomplete firmware provenance", id)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("operation %q is duplicated", id)
		}
		seen[id] = struct{}{}

		switch operation.Phase {
		case 3:
			syncCount++
		case StreamPhase:
			streamCount++
		default:
			return fmt.Errorf("operation %q has unsupported owner phase %d", id, operation.Phase)
		}
	}
	if syncCount != ExpectedSyncOperationCount {
		return fmt.Errorf("synchronous operation count = %d, want %d", syncCount, ExpectedSyncOperationCount)
	}
	if streamCount != 1 {
		return fmt.Errorf("stream operation count = %d, want 1", streamCount)
	}
	if operation, ok := c.Operation("GET /api/status/ws"); !ok || operation.Phase != StreamPhase {
		return fmt.Errorf("GET /api/status/ws must be owned by phase %d", StreamPhase)
	}
	return nil
}

// Validate checks the recorded status-stream contract.
func (c StatusStreamContract) Validate() error {
	if c.Path != "/api/status/ws" || c.AccessKeyQuery != "x-api-token" || c.APISemVerQuery != "x-api-sem-ver" {
		return fmt.Errorf("status stream address or query contract is invalid")
	}
	if c.InitialControl != `{"enable":true,"send":"all"}` || c.SnapshotControl != `{"send":"all"}` {
		return fmt.Errorf("status stream controls are invalid")
	}
	if c.MaxClients != 4 || c.FrameIntervalMS != 100 || c.PublisherHeartbeatMS != 991 || c.ClientHeartbeatIntervalMS != 10000 {
		return fmt.Errorf("status stream limits or timing are invalid")
	}
	if c.RateLimitMaxPackets != 11 || c.RateLimitPeriodMS != 1000 {
		return fmt.Errorf("status stream rate limit is invalid")
	}
	if c.StateUpdateKinds != ExpectedStreamUpdateKinds {
		return fmt.Errorf("status stream update kinds = %d, want %d", c.StateUpdateKinds, ExpectedStreamUpdateKinds)
	}
	if !c.FrontFramesOnly || c.FatalErrorCause != "RESOURCE_LIMIT" {
		return fmt.Errorf("status stream frame or fatal-error contract is invalid")
	}
	if len(c.SourceReferences) != 4 {
		return fmt.Errorf("status stream source reference count = %d, want 4", len(c.SourceReferences))
	}
	for _, reference := range c.SourceReferences {
		if reference.SourceFile == "" || reference.SourceSymbol == "" {
			return fmt.Errorf("status stream source provenance is incomplete")
		}
	}
	return nil
}

// Validate checks the recorded frame contract.
func (c FrameContract) Validate() error {
	if c.HTTPPath != "/api/screen" || c.HTTPEncoding != "base64" || c.MaxPayloadBytes != 16_384 {
		return fmt.Errorf("frame HTTP path or payload limit is invalid")
	}
	if !slices.Equal(c.EmittedEncodings, []string{"PLAIN", "RUN_LENGTH"}) {
		return fmt.Errorf("firmware-emitted frame encodings are invalid")
	}
	if !slices.Equal(c.ProtobufPixelFormats, []string{"RGB888", "L8", "L4"}) {
		return fmt.Errorf("frame protobuf pixel formats are invalid")
	}
	wantFront := FrameSurfaceContract{
		Screen:        0,
		Width:         72,
		Height:        16,
		PixelFormat:   "RGB888",
		PlainBytes:    3456,
		WireLayout:    "BGR",
		RLEBlockBytes: 3,
	}
	wantBack := FrameSurfaceContract{
		Screen:        1,
		Width:         160,
		Height:        80,
		PixelFormat:   "L4",
		PlainBytes:    6400,
		WireLayout:    "L4_LOW_NIBBLE_FIRST",
		RLEBlockBytes: 2,
	}
	if c.Front != wantFront || c.Back != wantBack {
		return fmt.Errorf("front or back frame contract is invalid")
	}
	if len(c.SourceReferences) != 8 {
		return fmt.Errorf("frame source reference count = %d, want 8", len(c.SourceReferences))
	}
	for _, reference := range c.SourceReferences {
		if reference.SourceFile == "" || reference.SourceSymbol == "" {
			return fmt.Errorf("frame source provenance is incomplete")
		}
	}
	return nil
}

// Validate checks the recorded snapshot contract.
func (c SnapshotContract) Validate() error {
	wantHTTP := []SnapshotHTTPContract{
		{Section: "name", Path: "/api/name", CanonicalKeys: []string{"name"}},
		{Section: "version", Path: "/api/version", CanonicalKeys: []string{"api_semver"}},
		{Section: "status", Path: "/api/status", CanonicalKeys: []string{"device", "firmware", "system", "power"}},
		{Section: "system", Path: "/api/status/system", CanonicalKeys: []string{"api_semver", "uptime", "boot_time", "auto_update_enabled"}},
		{Section: "power", Path: "/api/status/power", CanonicalKeys: []string{"state", "battery_charge", "battery_voltage", "battery_current", "usb_voltage"}},
		{Section: "time", Path: "/api/time", CanonicalKeys: []string{"timestamp"}},
		{Section: "wifi", Path: "/api/wifi/status", CanonicalKeys: []string{"state", "ssid", "bssid", "channel", "rssi", "security", "ip_config"}},
		{Section: "brightness", Path: "/api/display/brightness", CanonicalKeys: []string{"value"}},
		{Section: "audio_volume", Path: "/api/audio/volume", CanonicalKeys: []string{"volume"}},
		{Section: "ble", Path: "/api/ble/status", CanonicalKeys: []string{"status", "address"}},
		{Section: "storage", Path: "/api/storage/status", CanonicalKeys: []string{"used_bytes", "free_bytes", "total_bytes"}},
	}
	if len(c.HTTP) != len(wantHTTP) {
		return fmt.Errorf("snapshot HTTP section count = %d, want %d", len(c.HTTP), len(wantHTTP))
	}
	for index, want := range wantHTTP {
		got := c.HTTP[index]
		if got.Section != want.Section || got.Path != want.Path || !slices.Equal(got.CanonicalKeys, want.CanonicalKeys) {
			return fmt.Errorf("snapshot HTTP section %d is invalid", index)
		}
	}
	wantKinds := []string{
		"device_name",
		"power",
		"brightness",
		"audio_volume",
		"wifi",
		"update_state",
		"update_check",
		"timezone",
		"matter",
		"frame",
		"input",
		"timer",
		"ble",
		"auto_update_state",
		"timer_profiles",
	}
	if !slices.Equal(c.StateUpdateKinds, wantKinds) {
		return fmt.Errorf("snapshot state update kinds are invalid")
	}
	if len(c.SourceReferences) != 10 {
		return fmt.Errorf("snapshot source reference count = %d, want 10", len(c.SourceReferences))
	}
	for _, reference := range c.SourceReferences {
		if reference.SourceFile == "" || reference.SourceSymbol == "" {
			return fmt.Errorf("snapshot source provenance is incomplete")
		}
	}
	return nil
}

// Validate checks all recorded optional-tool contracts.
func (c OptionalToolsContract) Validate() error {
	if err := c.CLI.Validate(); err != nil {
		return err
	}
	return c.Media.Validate()
}

// Validate checks the recorded remote MQTT contract.
func (c RemoteContract) Validate() error {
	if c.MQTTVersion != 5 || c.APIVersion != "v1" || c.TopicPattern != "sessions/{session_id}/{direction}/v1/{topic}" ||
		c.DownDirection != "down" || c.UpDirection != "up" {
		return fmt.Errorf("remote MQTT routing contract is invalid")
	}
	wantBlocked := []string{
		"POST /api/update",
		"DELETE /api/account",
		"POST /api/account/link",
		"PUT /api/account/backend",
		"POST /api/wifi/connect",
		"POST /api/wifi/disconnect",
		"GET /api/wifi/networks",
	}
	if c.HTTP.RequestTopic != "http-request" || c.HTTP.LocalHost != "http://127.0.0.1" ||
		c.HTTP.PathPrefix != "/api/" || c.HTTP.TimeoutMS != 5_000 || c.HTTP.RequestQoS != 2 ||
		c.HTTP.ResponseQoS != 1 || c.HTTP.InvalidStatus != 422 || !c.HTTP.RequiresResponseTopic ||
		!c.HTTP.RequiresCorrelationData || !c.HTTP.EchoesCorrelationData ||
		!slices.Equal(c.HTTP.BlockedOperations, wantBlocked) {
		return fmt.Errorf("remote HTTP contract is invalid")
	}
	if c.Stream.RequestTopic != "stream-request" || c.Stream.RequestQoS != 1 || c.Stream.ResponseQoS != 0 ||
		c.Stream.DefaultExpirySeconds != 60 || c.Stream.FrameIntervalMS != 500 || c.Stream.QueueSize != 4 ||
		!c.Stream.EmptyPayloadStops || !c.Stream.NonEmptyPayloadStarts || c.Stream.SnapshotOnStart ||
		!c.Stream.SinglePublisher || c.Stream.MessageLimitMaxCountKey != "max_count" ||
		c.Stream.MessageLimitIntervalSecondsKey != "interval_s" {
		return fmt.Errorf("remote stream contract is invalid")
	}
	if len(c.SourceReferences) != 7 {
		return fmt.Errorf("remote source reference count = %d, want 7", len(c.SourceReferences))
	}
	return validateSourceReferences("remote", c.SourceReferences)
}

// Validate checks the recorded USB CLI contract.
func (c CLIContract) Validate() error {
	if c.DefaultAddress != "10.0.4.20" || c.Port != 23 || c.Prompt != ">: " || c.InterruptByte != 3 {
		return fmt.Errorf("optional CLI transport contract is invalid")
	}
	if c.RebootCommand != "power reboot sw" {
		return fmt.Errorf("optional CLI reboot command is invalid")
	}
	want := []struct {
		name string
		mode string
	}{
		{"uptime", "buffered"},
		{"power", "buffered"},
		{"storage", "buffered"},
		{"update", "buffered"},
		{"input", "buffered"},
		{"loader", "buffered"},
		{"top", "stream"},
		{"free", "buffered"},
		{"free_blocks", "buffered"},
		{"log", "stream"},
		{"echo", "buffered"},
		{"device_info", "buffered"},
		{"date", "buffered"},
		{"timezone", "buffered"},
		{"matter", "buffered"},
		{"audio", "buffered"},
		{"display", "buffered"},
		{"sysctl", "buffered"},
		{"log_dump", "buffered"},
	}
	if len(c.Commands) != len(want) {
		return fmt.Errorf("optional CLI command count = %d, want %d", len(c.Commands), len(want))
	}
	for index, expected := range want {
		command := c.Commands[index]
		if command.Name != expected.name || command.Mode != expected.mode || command.SourceFile == "" || command.SourceSymbol == "" {
			return fmt.Errorf("optional CLI command %d is invalid", index)
		}
	}
	if len(c.SourceReferences) != 4 {
		return fmt.Errorf("optional CLI source reference count = %d, want 4", len(c.SourceReferences))
	}
	return validateSourceReferences("optional CLI", c.SourceReferences)
}

// Validate checks the recorded image and audio contracts.
func (c MediaContract) Validate() error {
	wantImage := ImageConversionContract{
		OutputFormat:   "PNG",
		Decoder:        "LODEPNG",
		FrontMaxWidth:  72,
		FrontMaxHeight: 16,
		BackMaxWidth:   160,
		BackMaxHeight:  80,
	}
	if c.Image != wantImage {
		return fmt.Errorf("optional image conversion contract is invalid")
	}
	wantAudio := AudioConversionContract{
		Header:          "none",
		Channels:        1,
		SampleRateHz:    44_100,
		BitsPerSample:   16,
		ByteOrder:       "little_endian",
		OutputExtension: ".snd",
	}
	if c.Audio != wantAudio {
		return fmt.Errorf("optional audio conversion contract is invalid")
	}
	if len(c.SourceReferences) != 4 {
		return fmt.Errorf("optional media source reference count = %d, want 4", len(c.SourceReferences))
	}
	return validateSourceReferences("optional media", c.SourceReferences)
}

func validateSourceReferences(section string, references []SourceReference) error {
	for _, reference := range references {
		if reference.SourceFile == "" || reference.SourceSymbol == "" {
			return fmt.Errorf("%s source provenance is incomplete", section)
		}
	}
	return nil
}

// Operation returns the operation with id, where id is "METHOD /path".
func (c Contract) Operation(id string) (Operation, bool) {
	for _, operation := range c.Operations {
		if operation.ID() == id {
			return operation, true
		}
	}
	return Operation{}, false
}

// OperationIDs returns all operation IDs in lexical order.
func (c Contract) OperationIDs() []string {
	ids := make([]string, 0, len(c.Operations))
	for _, operation := range c.Operations {
		ids = append(ids, operation.ID())
	}
	sort.Strings(ids)
	return ids
}
