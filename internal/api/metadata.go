package api

import (
	"sort"
	"strings"
)

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
