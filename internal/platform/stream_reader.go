package platform

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// StreamReader reads NDJSON lines from Claude Code --output-format stream-json.
// Invalid JSON lines and empty lines are skipped (forward-compatible).
// Uses bufio.Reader.ReadBytes to avoid line size limits.
type StreamReader struct {
	reader  *bufio.Reader
	readErr error
	logger  interface{ Warn(string, ...any) }
}

// NewStreamReader creates a StreamReader from an io.Reader.
func NewStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{reader: bufio.NewReaderSize(r, 256*1024)}
}

// SetLogger sets an optional logger for parse error warnings.
func (sr *StreamReader) SetLogger(l interface{ Warn(string, ...any) }) {
	sr.logger = l
}

// Next returns the next parsed StreamMessage, skipping empty and invalid lines.
// Returns io.EOF when the stream ends cleanly.
// Returns the underlying read error for I/O failures (not io.EOF).
func (sr *StreamReader) Next() (*StreamMessage, error) {
	if sr.readErr != nil {
		err := sr.readErr
		sr.readErr = nil
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	for {
		line, err := sr.reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				msg, parseErr := ParseStreamMessage(trimmed)
				if parseErr != nil {
					if sr.logger != nil {
						sr.logger.Warn("stream-json parse skip", "error", parseErr)
					}
					if err != nil {
						if errors.Is(err, io.EOF) {
							return nil, io.EOF
						}
						return nil, err
					}
					continue
				}
				if err != nil && !errors.Is(err, io.EOF) {
					sr.readErr = err
				} else if errors.Is(err, io.EOF) {
					sr.readErr = io.EOF
				}
				return msg, nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			return nil, err
		}
	}
}

// CollectAll reads all messages until EOF and returns the result message
// (type=="result") and the full list of messages.
func (sr *StreamReader) CollectAll() (*StreamMessage, []*StreamMessage, error) {
	var messages []*StreamMessage
	var result *StreamMessage
	for {
		msg, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return result, messages, err
		}
		messages = append(messages, msg)
		if msg.Type == "result" {
			result = msg
		}
	}
	return result, messages, nil
}
