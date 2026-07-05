package busylib

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

type Body interface {
	prepareBody() (*preparedBody, error)
}

type preparedBody struct {
	contentType   string
	contentLength int64
	repeatable    bool
	open          func() (io.ReadCloser, error)
}

// JSONBody encodes value as an application/json request body.
// JSON bodies are repeatable and can be replayed for transport retries and
// API-semver compatibility retries.
func JSONBody(value any) Body {
	return jsonBody{value: value}
}

// BytesBody sends data with contentType.
// The bytes are copied, so the body is repeatable.
func BytesBody(data []byte, contentType string) Body {
	copied := append([]byte(nil), data...)
	return bytesBody{
		data:        copied,
		contentType: contentType,
	}
}

// ReaderBody sends reader as a single-use request body.
// It is never replayed for retries; use RepeatableBody or BytesBody when an
// upload must be retryable.
func ReaderBody(reader io.Reader, contentType string) Body {
	return &readerBody{
		reader:      reader,
		contentType: contentType,
	}
}

// RepeatableBody sends a body that can be opened again for each attempt.
// The opener must return a fresh reader every time it is called.
func RepeatableBody(contentType string, contentLength int64, open func() (io.ReadCloser, error)) Body {
	return repeatableBody{
		contentType:   contentType,
		contentLength: contentLength,
		open:          open,
	}
}

type jsonBody struct {
	value any
}

func (b jsonBody) prepareBody() (*preparedBody, error) {
	data, err := json.Marshal(b.value)
	if err != nil {
		return nil, err
	}
	return (&bytesBody{
		data:        data,
		contentType: "application/json; charset=utf-8",
	}).prepareBody()
}

type bytesBody struct {
	data        []byte
	contentType string
}

func (b bytesBody) prepareBody() (*preparedBody, error) {
	data := append([]byte(nil), b.data...)
	return &preparedBody{
		contentType:   b.contentType,
		contentLength: int64(len(data)),
		repeatable:    true,
		open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		},
	}, nil
}

type readerBody struct {
	reader      io.Reader
	contentType string

	mu   sync.Mutex
	used bool
}

func (b *readerBody) prepareBody() (*preparedBody, error) {
	if b.reader == nil {
		return nil, errors.New("reader body must not be nil")
	}
	return &preparedBody{
		contentType:   b.contentType,
		contentLength: -1,
		repeatable:    false,
		open: func() (io.ReadCloser, error) {
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.used {
				return nil, errors.New("reader body has already been consumed")
			}
			b.used = true
			if closer, ok := b.reader.(io.ReadCloser); ok {
				return closer, nil
			}
			return io.NopCloser(b.reader), nil
		},
	}, nil
}

type repeatableBody struct {
	contentType   string
	contentLength int64
	open          func() (io.ReadCloser, error)
}

func (b repeatableBody) prepareBody() (*preparedBody, error) {
	if b.open == nil {
		return nil, errors.New("repeatable body opener must not be nil")
	}
	return &preparedBody{
		contentType:   b.contentType,
		contentLength: b.contentLength,
		repeatable:    true,
		open:          b.open,
	}, nil
}

func emptyPreparedBody() *preparedBody {
	return &preparedBody{
		repeatable: true,
		open: func() (io.ReadCloser, error) {
			return nil, nil
		},
	}
}
