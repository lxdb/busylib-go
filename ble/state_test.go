package ble

import (
	"encoding/binary"
	"errors"
	"testing"
)

func TestStateAssemblerReassemblesOrderedPackets(t *testing.T) {
	assembler := newStateAssembler(32)
	first := statePacket(0, 2, []byte("hello "))
	second := statePacket(1, 2, []byte("world"))

	if message, complete, err := assembler.Push(first); err != nil || complete || message != nil {
		t.Fatalf("first Push = %q, %t, %v; want incomplete", message, complete, err)
	}
	message, complete, err := assembler.Push(second)
	if err != nil {
		t.Fatalf("second Push: %v", err)
	}
	if !complete || string(message) != "hello world" {
		t.Fatalf("second Push = %q, %t; want complete hello world", message, complete)
	}
}

func TestStateAssemblerRejectsOutOfSequencePacketAndResets(t *testing.T) {
	assembler := newStateAssembler(32)
	if _, _, err := assembler.Push(statePacket(0, 2, []byte("first"))); err != nil {
		t.Fatalf("first Push: %v", err)
	}
	if _, _, err := assembler.Push(statePacket(0, 3, []byte("restart"))); err != nil {
		t.Fatalf("restart Push: %v", err)
	}
	_, _, err := assembler.Push(statePacket(2, 3, []byte("wrong")))
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("out-of-sequence error = %v, want ErrProtocol", err)
	}
	message, complete, err := assembler.Push(statePacket(1, 3, []byte("ignored")))
	if err != nil || complete || message != nil {
		t.Fatalf("Push after reset = %q, %t, %v; want ignored", message, complete, err)
	}
}

func TestStateAssemblerRejectsMessageAboveLimit(t *testing.T) {
	assembler := newStateAssembler(4)
	_, _, err := assembler.Push(statePacket(0, 1, []byte("large")))
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("Push error = %v, want ErrMessageTooLarge", err)
	}
}

func statePacket(number, count uint16, payload []byte) []byte {
	packet := make([]byte, statePacketHeaderSize+len(payload))
	binary.LittleEndian.PutUint16(packet[0:2], number)
	binary.LittleEndian.PutUint16(packet[2:4], count)
	binary.LittleEndian.PutUint16(packet[4:6], uint16(len(payload)))
	copy(packet[statePacketHeaderSize:], payload)
	return packet
}
