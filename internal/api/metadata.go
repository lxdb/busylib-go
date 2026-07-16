package api

import (
	"sort"
	"strings"
)

// localOnlyOperations is a conservative proxy privacy policy. The local
// firmware contract does not define remote transport availability, so this
// list is intentionally kept separate from the firmware contract receipt.
var localOnlyOperations = map[string]struct{}{
	"DELETE /api/account":       {},
	"PUT /api/account/backend":  {},
	"POST /api/account/link":    {},
	"POST /api/wifi/connect":    {},
	"POST /api/wifi/disconnect": {},
	"GET /api/wifi/networks":    {},
}

func IsLocalOnlyOperation(method, path string) bool {
	_, ok := localOnlyOperations[operationID(method, path)]
	return ok
}

func LocalOnlyOperations() []string {
	operations := make([]string, 0, len(localOnlyOperations))
	for operation := range localOnlyOperations {
		operations = append(operations, operation)
	}
	sort.Slice(operations, func(i, j int) bool {
		leftMethod, leftPath := splitOperationID(operations[i])
		rightMethod, rightPath := splitOperationID(operations[j])
		if leftPath != rightPath {
			return leftPath < rightPath
		}
		return methodRank(leftMethod) < methodRank(rightMethod)
	})
	return operations
}

func operationID(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

func splitOperationID(operation string) (string, string) {
	method, path, ok := strings.Cut(operation, " ")
	if !ok {
		return operation, ""
	}
	return method, path
}

func methodRank(method string) int {
	switch method {
	case "DELETE":
		return 0
	case "GET":
		return 1
	case "PATCH":
		return 2
	case "POST":
		return 3
	case "PUT":
		return 4
	default:
		return 100
	}
}
