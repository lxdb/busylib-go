//go:build darwin && cgo

package ble

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tinygo-org/cbgo"
)

var (
	busyBarServiceUUID = cbgo.MustParseUUID("308A")
	nusServiceUUID     = cbgo.MustParseUUID("6E400001-B5A3-F393-E0A9-E50E24DCCA9E")
	nusRXUUID          = cbgo.MustParseUUID("6E400002-B5A3-F393-E0A9-E50E24DCCA9E")
	nusTXUUID          = cbgo.MustParseUUID("6E400003-B5A3-F393-E0A9-E50E24DCCA9E")
	stateServiceUUID   = cbgo.MustParseUUID("FFE0")
	stateDataUUID      = cbgo.MustParseUUID("FFE1")
)

type coreBluetoothBackend struct{}

type scanDelegate struct {
	cbgo.CentralManagerDelegateBase
	states      chan cbgo.ManagerState
	peripherals chan Peripheral
}

func newScanDelegate() *scanDelegate {
	return &scanDelegate{
		states:      make(chan cbgo.ManagerState, 2),
		peripherals: make(chan Peripheral, 64),
	}
}

func (d *scanDelegate) CentralManagerDidUpdateState(manager cbgo.CentralManager) {
	sendLatest(d.states, manager.State())
}

func (d *scanDelegate) DidDiscoverPeripheral(
	_ cbgo.CentralManager,
	peripheral cbgo.Peripheral,
	advertisement cbgo.AdvFields,
	rssi int,
) {
	if advertisement.Connectable != nil && !*advertisement.Connectable {
		return
	}
	name := advertisement.LocalName
	if name == "" {
		name = peripheral.Name()
	}
	value := Peripheral{
		Identifier: Identifier(peripheral.Identifier().String()),
		Name:       name,
		RSSI:       rssi,
	}
	select {
	case d.peripherals <- value:
	default:
	}
}

func (coreBluetoothBackend) Scan(ctx context.Context, duration time.Duration) ([]Peripheral, error) {
	delegate := newScanDelegate()
	manager := cbgo.NewCentralManager(nil)
	manager.SetDelegate(delegate)
	defer manager.SetDelegate(nil)
	if err := waitForPoweredOn(ctx, manager, delegate.states, duration); err != nil {
		return nil, err
	}
	manager.Scan([]cbgo.UUID{busyBarServiceUUID}, &cbgo.CentralManagerScanOpts{AllowDuplicates: true})
	defer manager.StopScan()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	var peripherals []Peripheral
	for {
		select {
		case peripheral := <-delegate.peripherals:
			peripherals = append(peripherals, peripheral)
		case <-timer.C:
			return peripherals, nil
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		}
	}
}

func (coreBluetoothBackend) Connect(
	ctx context.Context,
	identifier Identifier,
	timeout time.Duration,
) (connection, error) {
	uuid, err := cbgo.ParseUUID(string(identifier))
	if err != nil {
		return nil, fmt.Errorf("parse CoreBluetooth identifier: %w", err)
	}
	delegate := newConnectionDelegate()
	manager := cbgo.NewCentralManager(nil)
	manager.SetDelegate(delegate)
	if err := waitForPoweredOn(ctx, manager, delegate.states, timeout); err != nil {
		manager.SetDelegate(nil)
		return nil, err
	}
	peripherals := manager.RetrievePeripheralsWithIdentifiers([]cbgo.UUID{uuid})
	if len(peripherals) == 0 {
		manager.SetDelegate(nil)
		return nil, ErrNotFound
	}
	instance := &coreConnection{
		uuid:       uuid,
		manager:    manager,
		peripheral: peripherals[0],
		delegate:   delegate,
		timeout:    timeout,
	}
	if err := instance.connectAndConfigure(ctx, timeout); err != nil {
		_ = instance.Close()
		return nil, err
	}
	return instance, nil
}

type characteristicResult struct {
	uuid string
	err  error
}

type connectionDelegate struct {
	cbgo.CentralManagerDelegateBase
	cbgo.PeripheralDelegateBase

	states          chan cbgo.ManagerState
	connected       chan error
	disconnected    chan error
	services        chan error
	characteristics chan characteristicResult
	notifications   chan characteristicResult
	writes          chan characteristicResult

	handlerMu         sync.RWMutex
	httpHandler       func([]byte)
	httpErrorHandler  func(error)
	stateHandler      func([]byte)
	stateErrorHandler func(error)
	disconnectHandler func(error)
}

func newConnectionDelegate() *connectionDelegate {
	return &connectionDelegate{
		states:          make(chan cbgo.ManagerState, 2),
		connected:       make(chan error, 2),
		disconnected:    make(chan error, 4),
		services:        make(chan error, 2),
		characteristics: make(chan characteristicResult, 4),
		notifications:   make(chan characteristicResult, 8),
		writes:          make(chan characteristicResult, 8),
	}
}

func (d *connectionDelegate) CentralManagerDidUpdateState(manager cbgo.CentralManager) {
	sendLatest(d.states, manager.State())
}

func (d *connectionDelegate) DidConnectPeripheral(_ cbgo.CentralManager, peripheral cbgo.Peripheral) {
	peripheral.SetDelegate(d)
	sendLatest(d.connected, error(nil))
}

func (d *connectionDelegate) DidFailToConnectPeripheral(
	_ cbgo.CentralManager,
	_ cbgo.Peripheral,
	err error,
) {
	sendLatest(d.connected, nativeError("connect", err))
}

func (d *connectionDelegate) DidDisconnectPeripheral(
	_ cbgo.CentralManager,
	_ cbgo.Peripheral,
	err error,
) {
	if err == nil {
		err = ErrDisconnected
	} else {
		err = nativeError("disconnect", err)
	}
	sendLatest(d.disconnected, err)
	d.handlerMu.RLock()
	handler := d.disconnectHandler
	d.handlerMu.RUnlock()
	if handler != nil {
		handler(err)
	}
}

func (d *connectionDelegate) DidDiscoverServices(_ cbgo.Peripheral, err error) {
	sendLatest(d.services, nativeError("discover services", err))
}

func (d *connectionDelegate) DidDiscoverCharacteristics(
	_ cbgo.Peripheral,
	service cbgo.Service,
	err error,
) {
	sendLatest(d.characteristics, characteristicResult{
		uuid: service.UUID().String(),
		err:  nativeError("discover characteristics", err),
	})
}

func (d *connectionDelegate) DidUpdateNotificationState(
	_ cbgo.Peripheral,
	characteristic cbgo.Characteristic,
	err error,
) {
	sendLatest(d.notifications, characteristicResult{
		uuid: characteristic.UUID().String(),
		err:  nativeError("change notification state", err),
	})
}

func (d *connectionDelegate) DidWriteValueForCharacteristic(
	_ cbgo.Peripheral,
	characteristic cbgo.Characteristic,
	err error,
) {
	sendLatest(d.writes, characteristicResult{
		uuid: characteristic.UUID().String(),
		err:  nativeError("write characteristic", err),
	})
}

func (d *connectionDelegate) DidUpdateValueForCharacteristic(
	_ cbgo.Peripheral,
	characteristic cbgo.Characteristic,
	err error,
) {
	if err != nil {
		d.handleCharacteristicError(characteristic.UUID(), err)
		return
	}
	payload := bytes.Clone(characteristic.Value())
	d.handlerMu.RLock()
	defer d.handlerMu.RUnlock()
	switch {
	case sameUUID(characteristic.UUID(), nusTXUUID):
		if d.httpHandler != nil {
			d.httpHandler(payload)
		}
	case sameUUID(characteristic.UUID(), stateDataUUID):
		if d.stateHandler != nil {
			d.stateHandler(payload)
		}
	}
}

func (d *connectionDelegate) handleCharacteristicError(uuid cbgo.UUID, err error) {
	receiveErr := nativeError("receive characteristic", err)
	d.handlerMu.RLock()
	defer d.handlerMu.RUnlock()
	switch {
	case sameUUID(uuid, nusTXUUID):
		if d.httpErrorHandler != nil {
			d.httpErrorHandler(receiveErr)
		}
	case sameUUID(uuid, stateDataUUID):
		if d.stateErrorHandler != nil {
			d.stateErrorHandler(receiveErr)
		}
	}
}

type coreConnection struct {
	uuid       cbgo.UUID
	manager    cbgo.CentralManager
	peripheral cbgo.Peripheral
	delegate   *connectionDelegate
	timeout    time.Duration

	opMu         sync.Mutex
	mu           sync.RWMutex
	rx           cbgo.Characteristic
	tx           cbgo.Characteristic
	state        cbgo.Characteristic
	closed       bool
	stateEnabled bool
}

func (c *coreConnection) connectAndConfigure(ctx context.Context, timeout time.Duration) error {
	drain(c.delegate.connected)
	drain(c.delegate.disconnected)
	c.peripheral.SetDelegate(c.delegate)
	c.manager.Connect(c.peripheral, nil)
	if err := waitError(ctx, c.delegate.connected, c.delegate.disconnected, timeout, "connect peripheral"); err != nil {
		c.manager.CancelConnect(c.peripheral)
		return err
	}
	return c.configure(ctx, timeout)
}

func (c *coreConnection) configure(ctx context.Context, timeout time.Duration) error {
	drain(c.delegate.services)
	c.peripheral.DiscoverServices([]cbgo.UUID{nusServiceUUID, stateServiceUUID})
	if err := waitError(ctx, c.delegate.services, c.delegate.disconnected, timeout, "discover services"); err != nil {
		return err
	}
	nus, ok := findService(c.peripheral.Services(), nusServiceUUID)
	if !ok {
		return &Error{Operation: "discover services", Err: fmt.Errorf("%w: Nordic UART Service is missing", ErrProtocol)}
	}
	stateService, ok := findService(c.peripheral.Services(), stateServiceUUID)
	if !ok {
		return &Error{Operation: "discover services", Err: fmt.Errorf("%w: FFE0 service is missing", ErrProtocol)}
	}

	drain(c.delegate.characteristics)
	c.peripheral.DiscoverCharacteristics([]cbgo.UUID{nusRXUUID, nusTXUUID}, nus)
	if err := waitCharacteristic(ctx, c.delegate.characteristics, c.delegate.disconnected, nusServiceUUID, timeout, "discover NUS characteristics"); err != nil {
		return err
	}
	c.peripheral.DiscoverCharacteristics([]cbgo.UUID{stateDataUUID}, stateService)
	if err := waitCharacteristic(ctx, c.delegate.characteristics, c.delegate.disconnected, stateServiceUUID, timeout, "discover FFE1 characteristic"); err != nil {
		return err
	}
	rx, rxOK := findCharacteristic(nus.Characteristics(), nusRXUUID)
	tx, txOK := findCharacteristic(nus.Characteristics(), nusTXUUID)
	state, stateOK := findCharacteristic(stateService.Characteristics(), stateDataUUID)
	if !rxOK || !txOK || !stateOK {
		return &Error{Operation: "discover characteristics", Err: fmt.Errorf("%w: required NUS or FFE1 characteristic is missing", ErrProtocol)}
	}
	c.mu.Lock()
	c.rx = rx
	c.tx = tx
	c.state = state
	c.stateEnabled = false
	c.mu.Unlock()

	drain(c.delegate.notifications)
	c.peripheral.SetNotify(true, tx)
	return waitCharacteristic(ctx, c.delegate.notifications, c.delegate.disconnected, nusTXUUID, timeout, "subscribe NUS TX")
}

func (c *coreConnection) MaximumWriteValueLength() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.peripheral.MaximumWriteValueLength(true)
}

func (c *coreConnection) SetHTTPNotificationHandler(handler func([]byte)) {
	c.delegate.handlerMu.Lock()
	c.delegate.httpHandler = handler
	c.delegate.handlerMu.Unlock()
}

func (c *coreConnection) SetHTTPErrorHandler(handler func(error)) {
	c.delegate.handlerMu.Lock()
	c.delegate.httpErrorHandler = handler
	c.delegate.handlerMu.Unlock()
}

func (c *coreConnection) SetStateErrorHandler(handler func(error)) {
	c.delegate.handlerMu.Lock()
	c.delegate.stateErrorHandler = handler
	c.delegate.handlerMu.Unlock()
}

func (c *coreConnection) SetDisconnectHandler(handler func(error)) {
	c.delegate.handlerMu.Lock()
	c.delegate.disconnectHandler = handler
	c.delegate.handlerMu.Unlock()
}

func (c *coreConnection) Write(ctx context.Context, fragment []byte) error {
	if err := c.acquireOperation(ctx); err != nil {
		return err
	}
	defer c.opMu.Unlock()
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClosed
	}
	peripheral := c.peripheral
	rx := c.rx
	timeout := c.timeout
	c.mu.RUnlock()
	if peripheral.State() != cbgo.PeripheralStateConnected {
		return ErrDisconnected
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	drain(c.delegate.writes)
	peripheral.WriteCharacteristic(fragment, rx, true)
	return waitCharacteristic(ctx, c.delegate.writes, c.delegate.disconnected, nusRXUUID, timeout, "write NUS RX")
}

func (c *coreConnection) acquireOperation(ctx context.Context) error {
	c.opMu.Lock()
	if err := ctx.Err(); err != nil {
		c.opMu.Unlock()
		return err
	}
	return nil
}

func (c *coreConnection) EnableStateNotifications(ctx context.Context, handler func([]byte)) error {
	if handler == nil {
		return errors.New("BLE state notification handler must not be nil")
	}
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClosed
	}
	peripheral := c.peripheral
	state := c.state
	timeout := c.timeout
	c.mu.RUnlock()
	c.delegate.handlerMu.Lock()
	c.delegate.stateHandler = handler
	c.delegate.handlerMu.Unlock()
	drain(c.delegate.notifications)
	peripheral.SetNotify(true, state)
	if err := waitCharacteristic(ctx, c.delegate.notifications, c.delegate.disconnected, stateDataUUID, timeout, "subscribe FFE1"); err != nil {
		c.delegate.handlerMu.Lock()
		c.delegate.stateHandler = nil
		c.delegate.handlerMu.Unlock()
		return err
	}
	c.mu.Lock()
	c.stateEnabled = true
	c.mu.Unlock()
	return nil
}

func (c *coreConnection) DisableStateNotifications(ctx context.Context) error {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.mu.RLock()
	enabled := c.stateEnabled
	closed := c.closed
	peripheral := c.peripheral
	state := c.state
	timeout := c.timeout
	c.mu.RUnlock()
	c.delegate.handlerMu.Lock()
	c.delegate.stateHandler = nil
	c.delegate.handlerMu.Unlock()
	if !enabled || closed || peripheral.State() != cbgo.PeripheralStateConnected {
		return nil
	}
	drain(c.delegate.notifications)
	peripheral.SetNotify(false, state)
	err := waitCharacteristic(ctx, c.delegate.notifications, c.delegate.disconnected, stateDataUUID, timeout, "unsubscribe FFE1")
	c.mu.Lock()
	c.stateEnabled = false
	c.mu.Unlock()
	return err
}

func (c *coreConnection) Reconnect(ctx context.Context, timeout time.Duration) error {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClosed
	}
	c.mu.RUnlock()
	if err := waitForPoweredOn(ctx, c.manager, c.delegate.states, timeout); err != nil {
		return err
	}
	peripherals := c.manager.RetrievePeripheralsWithIdentifiers([]cbgo.UUID{c.uuid})
	if len(peripherals) == 0 {
		return ErrNotFound
	}
	c.mu.Lock()
	c.peripheral = peripherals[0]
	c.mu.Unlock()
	return c.connectAndConfigure(ctx, timeout)
}

func (c *coreConnection) Close() error {
	c.opMu.Lock()
	defer c.opMu.Unlock()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	peripheral := c.peripheral
	tx := c.tx
	state := c.state
	stateEnabled := c.stateEnabled
	c.stateEnabled = false
	c.mu.Unlock()
	c.delegate.handlerMu.Lock()
	c.delegate.httpHandler = nil
	c.delegate.httpErrorHandler = nil
	c.delegate.stateHandler = nil
	c.delegate.stateErrorHandler = nil
	c.delegate.disconnectHandler = nil
	c.delegate.handlerMu.Unlock()
	if peripheral.State() == cbgo.PeripheralStateConnected {
		if stateEnabled {
			peripheral.SetNotify(false, state)
		}
		peripheral.SetNotify(false, tx)
		c.manager.CancelConnect(peripheral)
	} else if peripheral.State() == cbgo.PeripheralStateConnecting {
		c.manager.CancelConnect(peripheral)
	}
	c.manager.SetDelegate(nil)
	return nil
}

func waitForPoweredOn(
	ctx context.Context,
	manager cbgo.CentralManager,
	states <-chan cbgo.ManagerState,
	timeout time.Duration,
) error {
	if state := manager.State(); state != cbgo.ManagerStateUnknown {
		return validateManagerState(state)
	}
	waitCtx, cancel := context.WithTimeoutCause(ctx, timeout, errors.New("timed out waiting for the Bluetooth adapter"))
	defer cancel()
	select {
	case state := <-states:
		return validateManagerState(state)
	case <-waitCtx.Done():
		return context.Cause(waitCtx)
	}
}

func validateManagerState(state cbgo.ManagerState) error {
	if state == cbgo.ManagerStatePoweredOn {
		return nil
	}
	return &Error{Operation: "use adapter", Err: fmt.Errorf("bluetooth adapter state is %d", state)}
}

func waitError(
	ctx context.Context,
	callbacks <-chan error,
	disconnected <-chan error,
	timeout time.Duration,
	operation string,
) error {
	waitCtx, cancel := context.WithTimeoutCause(ctx, timeout, fmt.Errorf("timed out waiting to %s", operation))
	defer cancel()
	select {
	case err := <-callbacks:
		return err
	case err := <-disconnected:
		return err
	case <-waitCtx.Done():
		return context.Cause(waitCtx)
	}
}

func waitCharacteristic(
	ctx context.Context,
	callbacks <-chan characteristicResult,
	disconnected <-chan error,
	want cbgo.UUID,
	timeout time.Duration,
	operation string,
) error {
	waitCtx, cancel := context.WithTimeoutCause(ctx, timeout, fmt.Errorf("timed out waiting to %s", operation))
	defer cancel()
	for {
		select {
		case result := <-callbacks:
			if !strings.EqualFold(result.uuid, want.String()) {
				continue
			}
			return result.err
		case err := <-disconnected:
			return err
		case <-waitCtx.Done():
			return context.Cause(waitCtx)
		}
	}
}

func nativeError(operation string, err error) error {
	if err == nil {
		return nil
	}
	nativeCode := 0
	var native *cbgo.NSError
	if errors.As(err, &native) {
		nativeCode = native.Code()
	}
	return &Error{Operation: operation, NativeCode: nativeCode, Err: err}
}

func findService(services []cbgo.Service, want cbgo.UUID) (cbgo.Service, bool) {
	for _, service := range services {
		if sameUUID(service.UUID(), want) {
			return service, true
		}
	}
	return cbgo.Service{}, false
}

func findCharacteristic(characteristics []cbgo.Characteristic, want cbgo.UUID) (cbgo.Characteristic, bool) {
	for _, characteristic := range characteristics {
		if sameUUID(characteristic.UUID(), want) {
			return characteristic, true
		}
	}
	return cbgo.Characteristic{}, false
}

func sameUUID(left, right cbgo.UUID) bool {
	return strings.EqualFold(left.String(), right.String())
}

func drain[T any](channel <-chan T) {
	for {
		select {
		case <-channel:
		default:
			return
		}
	}
}

func sendLatest[T any](channel chan T, value T) {
	select {
	case channel <- value:
		return
	default:
	}
	select {
	case <-channel:
	default:
	}
	select {
	case channel <- value:
	default:
	}
}
