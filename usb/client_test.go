package usb

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClientSendCommandUsesFreshPromptFramedConnection(t *testing.T) {
	address, done := serveOnce(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = conn.Write([]byte{255, 251, 1})
		_, _ = io.WriteString(conn, "BUSY Bar\r\n>: ")
		if got := readLine(t, conn); got != "uptime\r\n" {
			t.Errorf("command = %q", got)
		}
		_, _ = io.WriteString(conn, "uptime\r\n\x1b[32m123 seconds\x1b[0m\r\n>: ")
	})
	client := newTestClient(t, address)

	response, err := client.SendCommand(context.Background(), "uptime")
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if response.Command != "uptime" || response.Output != "123 seconds" {
		t.Fatalf("response = %#v", response)
	}
	if !bytes.Contains(response.Raw, []byte("uptime\r\n")) || !bytes.HasSuffix(response.Raw, []byte(prompt)) {
		t.Fatalf("raw response = %q", response.Raw)
	}
	<-done
}

func TestClientProbeOnlyWaitsForPrompt(t *testing.T) {
	address, done := serveOnce(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, "ready\r\n>: ")
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		buffer := make([]byte, 1)
		if count, err := conn.Read(buffer); count != 0 || (err != nil && !isTimeout(err) && !errors.Is(err, io.EOF)) {
			t.Errorf("unexpected probe payload: count=%d err=%v", count, err)
		}
	})
	client := newTestClient(t, address)

	if err := client.Probe(context.Background()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	<-done
}

func TestClientRejectsCommandInjectionBeforeDial(t *testing.T) {
	client := newTestClient(t, "127.0.0.1:1")
	for _, command := range []string{"", "uptime\nreboot", "echo\x00bad"} {
		_, err := client.SendCommand(context.Background(), command)
		if !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("SendCommand(%q) error = %v", command, err)
		}
	}
}

func TestClientDoesNotMistakePromptTextInsideOutputForThePrompt(t *testing.T) {
	fragmentWritten := make(chan struct{})
	writePrompt := make(chan struct{})
	address, done := serveOnce(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, ">: ")
		_ = readLine(t, conn)
		_, _ = io.WriteString(conn, "echo marker\r\nvalue >: marker\r\n")
		close(fragmentWritten)
		<-writePrompt
		_, _ = io.WriteString(conn, ">: ")
	})
	client := newTestClient(t, address)

	type commandResult struct {
		response Response
		err      error
	}
	result := make(chan commandResult, 1)
	go func() {
		response, err := client.SendCommand(context.Background(), "echo", "marker")
		result <- commandResult{response: response, err: err}
	}()
	<-fragmentWritten
	select {
	case early := <-result:
		t.Fatalf("SendCommand returned before the terminal prompt: response=%#v error=%v", early.response, early.err)
	default:
	}
	close(writePrompt)
	completed := <-result
	response, err := completed.response, completed.err
	if err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if response.Output != "value >: marker" {
		t.Fatalf("output = %q", response.Output)
	}
	<-done
}

func TestClientStopsAtConfiguredResponseLimit(t *testing.T) {
	address, done := serveOnce(t, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		_, _ = io.WriteString(conn, ">: ")
		_ = readLine(t, conn)
		_, _ = io.WriteString(conn, strings.Repeat("x", 64))
	})
	client, err := NewClient(WithAddress(address), WithMaxResponseBytes(16), WithCommandTimeout(time.Second))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	response, err := client.SendCommand(context.Background(), "uptime")
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("SendCommand error = %v", err)
	}
	if len(response.Raw) <= 16 {
		t.Fatalf("partial raw response length = %d", len(response.Raw))
	}
	<-done
}

func TestNewClientValidatesOptions(t *testing.T) {
	tests := []Option{
		WithAddress(""),
		WithDialTimeout(0),
		WithCommandTimeout(-time.Second),
		WithMaxResponseBytes(0),
	}
	for _, option := range tests {
		if _, err := NewClient(option); err == nil {
			t.Fatal("NewClient accepted an invalid option")
		}
	}
}

func newTestClient(t *testing.T, address string) *Client {
	t.Helper()
	client, err := NewClient(
		WithAddress(address),
		WithDialTimeout(time.Second),
		WithCommandTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func serveOnce(t *testing.T, handler func(net.Conn)) (string, <-chan struct{}) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = listener.Close() }()
		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		handler(conn)
	}()
	return listener.Addr().String(), done
}

func readLine(t *testing.T, reader io.Reader) string {
	t.Helper()
	var buffer bytes.Buffer
	byteBuffer := make([]byte, 1)
	for !strings.HasSuffix(buffer.String(), "\r\n") {
		if _, err := io.ReadFull(reader, byteBuffer); err != nil {
			t.Fatalf("read line: %v", err)
		}
		buffer.WriteByte(byteBuffer[0])
	}
	return buffer.String()
}

func isTimeout(err error) bool {
	var netError net.Error
	return errors.As(err, &netError) && netError.Timeout()
}
