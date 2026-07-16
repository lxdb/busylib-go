package api

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	ExpectedAPIVersion         = "24.4.0"
	ExpectedOperationCount     = 68
	ExpectedSyncOperationCount = 67
	StreamPhase                = 6
)

// Contract is an independently recorded audit of the BUSY Bar firmware HTTP
// handlers. It contains contract facts and source provenance, not copied
// firmware implementation code.
type Contract struct {
	Repository     string      `json:"repository"`
	Branch         string      `json:"branch"`
	FirmwareCommit string      `json:"firmwareCommit"`
	APIVersion     string      `json:"apiVersion"`
	ProtobufCommit string      `json:"protobufCommit"`
	License        string      `json:"license"`
	Operations     []Operation `json:"operations"`
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
