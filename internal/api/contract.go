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
	ExpectedAPIVersion         = "24.4.0"
	ExpectedOperationCount     = 68
	ExpectedSyncOperationCount = 67
	StreamPhase                = 6
	ExpectedStreamUpdateKinds  = 15
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
	Operations     []Operation           `json:"operations"`
}

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

type FrameContract struct {
	HTTPPath             string               `json:"httpPath"`
	MaxPayloadBytes      int                  `json:"maxPayloadBytes"`
	EmittedEncodings     []string             `json:"emittedEncodings"`
	ProtobufPixelFormats []string             `json:"protobufPixelFormats"`
	Front                FrameSurfaceContract `json:"front"`
	Back                 FrameSurfaceContract `json:"back"`
	SourceReferences     []SourceReference    `json:"sourceReferences"`
}

type FrameSurfaceContract struct {
	Screen        int    `json:"screen"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	PixelFormat   string `json:"pixelFormat"`
	PlainBytes    int    `json:"plainBytes"`
	WireLayout    string `json:"wireLayout"`
	RLEBlockBytes int    `json:"rleBlockBytes"`
}

type SnapshotContract struct {
	HTTP             []SnapshotHTTPContract `json:"http"`
	StateUpdateKinds []string               `json:"stateUpdateKinds"`
	SourceReferences []SourceReference      `json:"sourceReferences"`
}

type SnapshotHTTPContract struct {
	Section       string   `json:"section"`
	Path          string   `json:"path"`
	CanonicalKeys []string `json:"canonicalKeys"`
}

type OptionalToolsContract struct {
	CLI   CLIContract   `json:"cli"`
	Media MediaContract `json:"media"`
}

type CLIContract struct {
	DefaultAddress   string               `json:"defaultAddress"`
	Port             int                  `json:"port"`
	Prompt           string               `json:"prompt"`
	InterruptByte    int                  `json:"interruptByte"`
	RebootCommand    string               `json:"rebootCommand"`
	Commands         []CLICommandContract `json:"commands"`
	SourceReferences []SourceReference    `json:"sourceReferences"`
}

type CLICommandContract struct {
	Name         string `json:"name"`
	Mode         string `json:"mode"`
	SourceFile   string `json:"sourceFile"`
	SourceSymbol string `json:"sourceSymbol"`
}

type MediaContract struct {
	Image            ImageConversionContract `json:"image"`
	Audio            AudioConversionContract `json:"audio"`
	SourceReferences []SourceReference       `json:"sourceReferences"`
}

type ImageConversionContract struct {
	OutputFormat   string `json:"outputFormat"`
	Decoder        string `json:"decoder"`
	FrontMaxWidth  int    `json:"frontMaxWidth"`
	FrontMaxHeight int    `json:"frontMaxHeight"`
	BackMaxWidth   int    `json:"backMaxWidth"`
	BackMaxHeight  int    `json:"backMaxHeight"`
}

type AudioConversionContract struct {
	Header          string `json:"header"`
	Channels        int    `json:"channels"`
	SampleRateHz    int    `json:"sampleRateHz"`
	BitsPerSample   int    `json:"bitsPerSample"`
	ByteOrder       string `json:"byteOrder"`
	OutputExtension string `json:"outputExtension"`
}

type SourceReference struct {
	SourceFile   string `json:"sourceFile"`
	SourceSymbol string `json:"sourceSymbol"`
}

type Operation struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Phase        int    `json:"phase"`
	SourceFile   string `json:"sourceFile"`
	SourceSymbol string `json:"sourceSymbol"`
}

func (o Operation) ID() string {
	return strings.ToUpper(o.Method) + " " + o.Path
}

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

func (c FrameContract) Validate() error {
	if c.HTTPPath != "/api/screen" || c.MaxPayloadBytes != 16_384 {
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

func (c OptionalToolsContract) Validate() error {
	if err := c.CLI.Validate(); err != nil {
		return err
	}
	return c.Media.Validate()
}

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

func (c Contract) Operation(id string) (Operation, bool) {
	for _, operation := range c.Operations {
		if operation.ID() == id {
			return operation, true
		}
	}
	return Operation{}, false
}

func (c Contract) OperationIDs() []string {
	ids := make([]string, 0, len(c.Operations))
	for _, operation := range c.Operations {
		ids = append(ids, operation.ID())
	}
	sort.Strings(ids)
	return ids
}
