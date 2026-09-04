package usb

import (
	"context"
	"errors"
	"io"
	"net"
)

// Client opens a fresh USB CLI connection for each direct operation. Call
// Open when several commands should share one persistent connection.
type Client struct {
	config config
}

// Response contains both the exact prompt-framed bytes returned by the device
// and a cleaned form suitable for ordinary command output. Raw is owned by the
// caller.
type Response struct {
	Command string
	Raw     []byte
	Output  string
}

// NewClient creates an independent optional USB CLI client. By default, it
// connects to DefaultAddress with a 2-second dial timeout, a 5-second command
// timeout, and a 1 MiB response limit. Nil options are ignored.
func NewClient(options ...Option) (*Client, error) {
	config := defaultConfig()
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	return &Client{config: config}, nil
}

// Probe opens a fresh connection, verifies the firmware CLI prompt, and closes
// the connection.
func (c *Client) Probe(ctx context.Context) error {
	session, err := c.Open(ctx)
	if err != nil {
		return err
	}
	return session.Close()
}

// Open creates a persistent, serialized CLI session. A failed transport or an
// unrecovered prompt closes the session; callers must explicitly open another.
func (c *Client) Open(ctx context.Context) (*Session, error) {
	dialer := net.Dialer{Timeout: c.config.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.config.address)
	if err != nil {
		return nil, wrapError("dial", c.config.address, "", err)
	}
	session := &Session{conn: conn, config: c.config}
	if err := session.waitForInitialPrompt(ctx); err != nil {
		_ = conn.Close()
		return nil, wrapError("open", c.config.address, "", err)
	}
	return session, nil
}

// SendCommand runs one bounded command over a fresh connection, then closes the
// connection before returning.
func (c *Client) SendCommand(ctx context.Context, command string, args ...string) (Response, error) {
	line, err := buildCommand(command, args...)
	if err != nil {
		return Response{Command: line}, wrapError("send", c.config.address, line, err)
	}
	session, err := c.Open(ctx)
	if err != nil {
		return Response{Command: line}, err
	}
	response, commandErr := session.sendLine(ctx, line)
	return response, errors.Join(commandErr, session.Close())
}

// StreamCommand writes one continuous command to dst over a fresh connection.
// Cancel the context to send the firmware's ETX interrupt byte and recover the
// prompt before closing the connection.
func (c *Client) StreamCommand(ctx context.Context, dst io.Writer, command string, args ...string) error {
	line, err := buildCommand(command, args...)
	if err != nil {
		return wrapError("stream", c.config.address, line, err)
	}
	session, err := c.Open(ctx)
	if err != nil {
		return err
	}
	commandErr := session.streamLine(ctx, dst, line)
	return errors.Join(commandErr, session.Close())
}

func (c *Client) reboot(ctx context.Context) error {
	session, err := c.Open(ctx)
	if err != nil {
		return err
	}
	return session.reboot(ctx)
}

// Commands returns the curated firmware command wrappers.
func (c *Client) Commands() Commands {
	return Commands{
		send:   c.SendCommand,
		stream: c.StreamCommand,
		reboot: c.reboot,
	}
}
