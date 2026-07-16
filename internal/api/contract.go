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
	Repository     string               `json:"repository"`
	Branch         string               `json:"branch"`
	FirmwareCommit string               `json:"firmwareCommit"`
	APIVersion     string               `json:"apiVersion"`
	ProtobufCommit string               `json:"protobufCommit"`
	License        string               `json:"license"`
	StatusStream   StatusStreamContract `json:"statusStream"`
	Frames         FrameContract        `json:"frames"`
	Operations     []Operation          `json:"operations"`
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
