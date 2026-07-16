package api

import (
	"sort"
	"strings"
)

// remoteBlockedOperations mirrors mqtt_http_proxy_blocklist in the canonical
// firmware. These operations must be rejected before publishing an MQTT request.
var remoteBlockedOperations = map[string]struct{}{
	"POST /api/update":          {},
	"DELETE /api/account":       {},
	"PUT /api/account/backend":  {},
	"POST /api/account/link":    {},
	"POST /api/wifi/connect":    {},
	"POST /api/wifi/disconnect": {},
	"GET /api/wifi/networks":    {},
}

func IsRemoteBlockedOperation(method, path string) bool {
	if path != "/api/" {
		path = strings.TrimSuffix(path, "/")
	}
	_, ok := remoteBlockedOperations[operationID(method, path)]
	return ok
}

func RemoteBlockedOperations() []string {
	operations := make([]string, 0, len(remoteBlockedOperations))
	for operation := range remoteBlockedOperations {
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
