package ble

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	statePacketHeaderSize = 6
	statePacketPayloadMax = 231
	statePacketRecordSize = statePacketHeaderSize + statePacketPayloadMax
)

// stateAssembler joins FFE1 records whose six-byte little-endian header holds
// the record number, record count, and payload size. A record numbered zero
// starts a message, and all subsequent records must arrive in order.
type stateAssembler struct {
	maximum       int
	active        bool
	expectedCount uint16
	nextNumber    uint16
	payload       []byte
}

func newStateAssembler(maximum int) *stateAssembler {
	return &stateAssembler{maximum: maximum}
}

func (a *stateAssembler) Push(packet []byte) ([]byte, bool, error) {
	if len(packet) < statePacketHeaderSize || len(packet) > statePacketRecordSize {
		a.reset()
		return nil, false, fmt.Errorf("%w: invalid FFE1 packet size %d", ErrProtocol, len(packet))
	}
	number := binary.LittleEndian.Uint16(packet[0:2])
	count := binary.LittleEndian.Uint16(packet[2:4])
	size := binary.LittleEndian.Uint16(packet[4:6])
	if count == 0 || number >= count || size > statePacketPayloadMax || int(size) > len(packet)-statePacketHeaderSize {
		a.reset()
		return nil, false, fmt.Errorf("%w: invalid FFE1 packet header", ErrProtocol)
	}
	if number == 0 {
		a.reset()
		a.active = true
		a.expectedCount = count
	}
	if !a.active {
		return nil, false, nil
	}
	if count != a.expectedCount || number != a.nextNumber {
		a.reset()
		return nil, false, fmt.Errorf("%w: invalid FFE1 packet sequence", ErrProtocol)
	}
	if a.maximum <= 0 || int(size) > a.maximum-len(a.payload) {
		a.reset()
		return nil, false, ErrMessageTooLarge
	}
	a.payload = append(a.payload, packet[statePacketHeaderSize:statePacketHeaderSize+int(size)]...)
	a.nextNumber++
	if a.nextNumber != a.expectedCount {
		return nil, false, nil
	}
	message := bytes.Clone(a.payload)
	a.reset()
	return message, true, nil
}

func (a *stateAssembler) reset() {
	a.active = false
	a.expectedCount = 0
	a.nextNumber = 0
	a.payload = nil
}
