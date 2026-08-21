package usb

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestSessionReusesConnectionAndBecomesTerminalAfterTransportFailure(t *testing.T) {
	var accepted atomic.Int32
	address, done := serveOnce(t, func(conn net.Conn) {
		accepted.Add(1)
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, ">: ")
		if got := readLine(t, conn); got != "uptime\r\n" {
			t.Errorf("first command = %q", got)
		}
		_, _ = io.WriteString(conn, "uptime\r\n1\r\n>: ")
		if got := readLine(t, conn); got != "free\r\n" {
			t.Errorf("second command = %q", got)
		}
	})
	client := newTestClient(t, address)
	session, err := client.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = session.Close() }()

	if response, err := session.SendCommand(context.Background(), "uptime"); err != nil || response.Output != "1" {
		t.Fatalf("first response = %#v, error = %v", response, err)
	}
	if _, err := session.SendCommand(context.Background(), "free"); err == nil {
		t.Fatal("second command succeeded after the server closed")
	}
	if _, err := session.SendCommand(context.Background(), "uptime"); !errors.Is(err, ErrClosed) {
		t.Fatalf("terminal session error = %v", err)
	}
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted connections = %d, want 1", got)
	}
	<-done
}

func TestStreamCommandSendsETXAndRecoversPromptOnCancellation(t *testing.T) {
	etx := make(chan byte, 1)
	address, done := serveOnce(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, ">: ")
		if got := readLine(t, conn); got != "log\r\n" {
			t.Errorf("command = %q", got)
		}
		_, _ = io.WriteString(conn, "first line\r\n")
		buffer := make([]byte, 1)
		if _, err := io.ReadFull(conn, buffer); err != nil {
			t.Errorf("read ETX: %v", err)
			return
		}
		etx <- buffer[0]
		_, _ = io.WriteString(conn, "\r\n>: ")
		if got := readLine(t, conn); got != "uptime\r\n" {
			t.Errorf("post-recovery command = %q", got)
		}
		_, _ = io.WriteString(conn, "uptime\r\n7\r\n>: ")
	})
	client := newTestClient(t, address)
	session, err := client.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = session.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	writer := &cancelWriter{cancel: cancel}
	err = session.StreamCommand(ctx, writer, "log")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StreamCommand error = %v", err)
	}
	if got := <-etx; got != InterruptByte {
		t.Fatalf("interrupt byte = %d, want %d", got, InterruptByte)
	}
	if !bytes.Contains(writer.Bytes(), []byte("first line")) {
		t.Fatalf("stream output = %q", writer.Bytes())
	}
	response, err := session.SendCommand(context.Background(), "uptime")
	if err != nil || response.Output != "7" {
		t.Fatalf("post-recovery response = %#v, error = %v", response, err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	<-done
}

func TestSessionCloseUnblocksCommand(t *testing.T) {
	commandRead := make(chan struct{})
	address, done := serveOnce(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, ">: ")
		_ = readLine(t, conn)
		close(commandRead)
		_, _ = io.Copy(io.Discard, conn)
	})
	client := newTestClient(t, address)
	session, err := client.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := session.SendCommand(context.Background(), "uptime")
		result <- err
	}()
	<-commandRead
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("command succeeded after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock the command")
	}
	<-done
}

type cancelWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
	done   bool
}

func (w *cancelWriter) Write(data []byte) (int, error) {
	count, err := w.Buffer.Write(data)
	if !w.done {
		w.done = true
		w.cancel()
	}
	return count, err
}
