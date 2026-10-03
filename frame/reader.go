package frame

import (
	"bufio"
	"errors"
	"io"
)

// LineReader splits a byte stream into lines under the section 4 line-level rules:
// the first partial line after opening is discarded (unless the reader is told the
// stream starts at a frame boundary), and an over-long line is discarded up to and
// including its next LF.
type LineReader struct {
	r         *bufio.Reader
	skipFirst bool
	// Discarded counts lines dropped by the reader itself (partial start, too long).
	Discarded int
}

// NewLineReader wraps r. Set atBoundary when the stream is known to start at a frame
// boundary (e.g. a pipe opened before the producer started); otherwise the first line
// is discarded as possibly partial.
func NewLineReader(r io.Reader, atBoundary bool) *LineReader {
	return &LineReader{r: bufio.NewReaderSize(r, 4096), skipFirst: !atBoundary}
}

// Next returns the next complete line without its LF. It returns io.EOF when the
// stream ends; a trailing unterminated fragment is discarded.
func (lr *LineReader) Next() (string, error) {
	for {
		line, tooLong, err := lr.readLine()
		if err != nil {
			return "", err
		}
		if lr.skipFirst {
			lr.skipFirst = false
			lr.Discarded++
			continue
		}
		if tooLong {
			lr.Discarded++
			continue
		}
		return line, nil
	}
}

func (lr *LineReader) readLine() (string, bool, error) {
	var buf []byte
	tooLong := false
	for {
		b, err := lr.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && len(buf) > 0 {
				lr.Discarded++
			}
			return "", false, err
		}
		if b == '\n' {
			return string(buf), tooLong, nil
		}
		if len(buf)+1 >= MaxLineLen { // +1 for the LF that must still fit
			tooLong = true
			continue
		}
		buf = append(buf, b)
	}
}
