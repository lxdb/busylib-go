package busylib

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
)

type Body interface {
	prepareBody() (*preparedBody, error)
}

// ProgressFunc reports bytes read from a request body for the current attempt.
// total is -1 when the body length is unknown. A retry starts written at zero.
type ProgressFunc func(written, total int64)

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

// FileBody sends a local file as a repeatable request body.
func FileBody(path, contentType string) Body {
	return fileBody{
		path:        path,
		contentType: contentType,
	}
}

// ProgressBody reports upload progress while preserving the wrapped body's
// repeatability and content length.
func ProgressBody(body Body, onProgress ProgressFunc) Body {
	return progressBody{
		body:       body,
		onProgress: onProgress,
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

type fileBody struct {
	path        string
	contentType string
}

func (b fileBody) prepareBody() (*preparedBody, error) {
	if b.path == "" {
		return nil, errors.New("file body path must not be empty")
	}
	info, err := os.Stat(b.path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("file body path must be a file")
	}
	return &preparedBody{
		contentType:   b.contentType,
		contentLength: info.Size(),
		repeatable:    true,
		open: func() (io.ReadCloser, error) {
			return os.Open(b.path)
		},
	}, nil
}

type progressBody struct {
	body       Body
	onProgress ProgressFunc
}

func (b progressBody) prepareBody() (*preparedBody, error) {
	if b.body == nil {
		return nil, errors.New("progress body must wrap a body")
	}
	prepared, err := b.body.prepareBody()
	if err != nil {
		return nil, err
	}
	if b.onProgress == nil {
		return prepared, nil
	}
	return &preparedBody{
		contentType:   prepared.contentType,
		contentLength: prepared.contentLength,
		repeatable:    prepared.repeatable,
		open: func() (io.ReadCloser, error) {
			reader, err := prepared.open()
			if err != nil || reader == nil {
				return reader, err
			}
			return &progressReadCloser{
				reader:     reader,
				total:      prepared.contentLength,
				onProgress: b.onProgress,
			}, nil
		},
	}, nil
}

type progressReadCloser struct {
	reader     io.ReadCloser
	written    int64
	total      int64
	onProgress ProgressFunc
}

func (r *progressReadCloser) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.written += int64(n)
		r.onProgress(r.written, r.total)
	}
	return n, err
}

func (r *progressReadCloser) Close() error {
	return r.reader.Close()
}

func emptyPreparedBody() *preparedBody {
	return &preparedBody{
		repeatable: true,
		open: func() (io.ReadCloser, error) {
			return nil, nil
		},
	}
}
