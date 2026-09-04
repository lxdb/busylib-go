//go:build darwin && cgo

package ble

import (
	"context"
	"errors"
	"testing"
)

func TestCoreConnectionDoesNotOwnCanceledOperation(t *testing.T) {
	connection := new(coreConnection)
	connection.opMu.Lock()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- connection.acquireOperation(ctx) }()
	cancel()
	connection.opMu.Unlock()

	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireOperation error = %v, want context.Canceled", err)
	}
	if !connection.opMu.TryLock() {
		t.Fatal("operation mutex remained locked after canceled acquisition")
	}
	connection.opMu.Unlock()
}

func TestConnectionDelegateRoutesCharacteristicErrorsWithoutDisconnecting(t *testing.T) {
	cause := errors.New("notification failed")
	delegate := newConnectionDelegate()
	var httpErrors int
	var stateErrors int
	var disconnects int
	delegate.httpErrorHandler = func(err error) {
		if !errors.Is(err, cause) {
			t.Errorf("HTTP error = %v, want cause", err)
		}
		httpErrors++
	}
	delegate.stateErrorHandler = func(err error) {
		if !errors.Is(err, cause) {
			t.Errorf("state error = %v, want cause", err)
		}
		stateErrors++
	}
	delegate.disconnectHandler = func(error) { disconnects++ }

	delegate.handleCharacteristicError(nusTXUUID, cause)
	delegate.handleCharacteristicError(stateDataUUID, cause)

	if httpErrors != 1 || stateErrors != 1 || disconnects != 0 {
		t.Fatalf("HTTP errors = %d, state errors = %d, disconnects = %d; want 1, 1, 0", httpErrors, stateErrors, disconnects)
	}
}
