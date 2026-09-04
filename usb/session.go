package usb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Session is a persistent, serialized USB CLI connection. It permits one
// command at a time and becomes unusable after a transport or prompt-recovery
// failure.
type Session struct {
	conn   net.Conn
	config config

	opMu    sync.Mutex
	stateMu sync.Mutex
	closed  bool
}

func (s *Session) waitForInitialPrompt(ctx context.Context) error {
	stop := s.setDeadline(ctx, s.config.commandTimeout)
	defer stop()
	_, err := readUntilPrompt(s.conn, s.config.maxResponseBytes)
	if err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// SendCommand runs one bounded command on the persistent connection.
func (s *Session) SendCommand(ctx context.Context, command string, args ...string) (Response, error) {
	line, err := buildCommand(command, args...)
	if err != nil {
		return Response{Command: line}, wrapError("send", s.config.address, line, err)
	}
	return s.sendLine(ctx, line)
}

func (s *Session) sendLine(ctx context.Context, line string) (Response, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	response := Response{Command: line}
	if err := s.ensureOpen(); err != nil {
		return response, wrapError("send", s.config.address, line, err)
	}
	stop := s.setDeadline(ctx, s.config.commandTimeout)
	defer stop()
	if _, err := io.WriteString(s.conn, line+"\r\n"); err != nil {
		s.fail()
		return response, wrapError("send", s.config.address, line, contextOr(ctx, err))
	}
	raw, err := readUntilPrompt(s.conn, s.config.maxResponseBytes)
	response.Raw = raw
	response.Output = cleanOutput(raw, line)
	if err != nil {
		s.fail()
		return response, wrapError("send", s.config.address, line, contextOr(ctx, err))
	}
	return response, nil
}

// StreamCommand runs one continuous command on the persistent connection.
// Cancellation sends ETX and keeps the session usable only when the prompt is
// successfully recovered.
func (s *Session) StreamCommand(ctx context.Context, dst io.Writer, command string, args ...string) error {
	line, err := buildCommand(command, args...)
	if err != nil {
		return wrapError("stream", s.config.address, line, err)
	}
	return s.streamLine(ctx, dst, line)
}

func (s *Session) streamLine(ctx context.Context, dst io.Writer, line string) error {
	if dst == nil {
		return wrapError("stream", s.config.address, line, errors.New("USB CLI stream writer must not be nil"))
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return wrapError("stream", s.config.address, line, err)
	}
	stop := s.setDeadline(ctx, 0)
	if _, err := io.WriteString(s.conn, line+"\r\n"); err != nil {
		stop()
		s.fail()
		return wrapError("stream", s.config.address, line, contextOr(ctx, err))
	}

	promptTail := make([]byte, 0, len(prompt)-1)
	buffer := make([]byte, 4096)
	for {
		if err := ctx.Err(); err != nil {
			stop()
			return s.interruptAndRecover(line, err)
		}
		count, err := s.conn.Read(buffer)
		if count > 0 {
			chunk := buffer[:count]
			combined := append(append([]byte(nil), promptTail...), chunk...)
			written, writeErr := dst.Write(chunk)
			if writeErr == nil && written != len(chunk) {
				writeErr = io.ErrShortWrite
			}
			if writeErr != nil {
				stop()
				return s.interruptAndRecover(line, writeErr)
			}
			if containsPrompt(combined) {
				stop()
				return nil
			}
			promptTail = trailingBytes(combined, len(prompt)-1)
		}
		if err != nil {
			stop()
			if ctx.Err() != nil {
				return s.interruptAndRecover(line, ctx.Err())
			}
			s.fail()
			return wrapError("stream", s.config.address, line, err)
		}
	}
}

func (s *Session) interruptAndRecover(line string, cause error) error {
	_ = s.conn.SetDeadline(time.Now().Add(s.config.commandTimeout))
	if _, err := s.conn.Write([]byte{interruptByte}); err != nil {
		s.fail()
		return wrapError("interrupt", s.config.address, line, errors.Join(cause, fmt.Errorf("send ETX: %w", err)))
	}
	if _, err := readUntilPrompt(s.conn, s.config.maxResponseBytes); err != nil {
		s.fail()
		return wrapError("interrupt", s.config.address, line, errors.Join(cause, fmt.Errorf("recover prompt: %w", err)))
	}
	_ = s.conn.SetDeadline(time.Time{})
	return wrapError("stream", s.config.address, line, cause)
}

func (s *Session) reboot(ctx context.Context) error {
	const line = "power reboot sw"
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return wrapError("reboot", s.config.address, line, err)
	}
	stop := s.setDeadline(ctx, s.config.commandTimeout)
	_, err := io.WriteString(s.conn, line+"\r\n")
	stop()
	s.fail()
	return wrapError("reboot", s.config.address, line, contextOr(ctx, err))
}

// Commands returns the curated firmware command wrappers on this session.
func (s *Session) Commands() Commands {
	return Commands{
		send:   s.SendCommand,
		stream: s.StreamCommand,
		reboot: s.reboot,
	}
}

// Close is idempotent and unblocks an active command. It does not wait for that
// command's goroutine to return.
func (s *Session) Close() error {
	s.stateMu.Lock()
	if s.closed {
		s.stateMu.Unlock()
		return nil
	}
	s.closed = true
	s.stateMu.Unlock()
	_ = s.conn.Close()
	return nil
}

func (s *Session) ensureOpen() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.closed {
		return ErrClosed
	}
	return nil
}

func (s *Session) fail() {
	_ = s.Close()
}

func (s *Session) setDeadline(ctx context.Context, timeout time.Duration) func() {
	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if contextDeadline, ok := ctx.Deadline(); ok && (deadline.IsZero() || contextDeadline.Before(deadline)) {
		deadline = contextDeadline
	}
	_ = s.conn.SetDeadline(deadline)
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = s.conn.SetDeadline(time.Now())
		close(done)
	})
	return func() {
		if !stop() {
			<-done
		}
		_ = s.conn.SetDeadline(time.Time{})
	}
}

func contextOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
