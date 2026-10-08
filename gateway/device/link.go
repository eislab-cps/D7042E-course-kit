package device

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ErrNoLink is returned when no device is connected to write a downlink frame to.
var ErrNoLink = errors.New("device link down")

// writeTimeout bounds one downlink write, so a stalled link cannot block a caller.
const writeTimeout = 2 * time.Second

// Link is the write side of the device connection (FRAME_FORMAT.md section 7): the
// same serial device or TCP connection the frames arrive on, or, when the uplink is a
// named pipe, the second pipe <pipe>.down. RunLink attaches it while the source is
// open. It writes whole lines only; framing (C, BOOT, CID, the tag) is the caller's.
// Safe for concurrent use.
type Link struct {
	mu   sync.Mutex
	w    io.Writer // serial device or TCP connection; nil while detached
	pipe string    // pipe mode: <pipe>.down, opened on each write
	up   bool
}

// Up reports whether the source is open, so downlink frames have somewhere to go.
// In pipe mode a write can still fail with ErrNoLink if nothing reads <pipe>.down.
func (l *Link) Up() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.up
}

// WriteLine writes one downlink frame. A missing final LF is added. It returns
// ErrNoLink when the source is closed or, in pipe mode, nothing reads <pipe>.down.
func (l *Link) WriteLine(line string) error {
	if strings.ContainsAny(strings.TrimSuffix(line, "\n"), "\r\n") {
		return fmt.Errorf("downlink frame must be one line: %q", line)
	}
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.up {
		return ErrNoLink
	}
	if l.pipe != "" {
		return writePipe(l.pipe, line)
	}
	if d, ok := l.w.(interface{ SetWriteDeadline(time.Time) error }); ok {
		d.SetWriteDeadline(time.Now().Add(writeTimeout)) // a pty or serial device may not support it
	}
	_, err := io.WriteString(l.w, line)
	return err
}

// writePipe opens the downlink pipe write-only and non-blocking for one frame. Holding
// it open, or opening it read-write, would let the pipe buffer frames while no device
// reads them and deliver stale commands to the next simulator that opens it.
func writePipe(path, line string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, syscall.ENXIO) || errors.Is(err, os.ErrNotExist) {
		return ErrNoLink // no reader yet
	}
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, line)
	if errors.Is(err, syscall.EPIPE) {
		return ErrNoLink
	}
	return err
}

func (l *Link) attach(src Source, rc io.ReadCloser) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.up = true
	if p := src.DownlinkPipe(); p != "" {
		if err := syscall.Mkfifo(p, 0o600); err != nil && !errors.Is(err, syscall.EEXIST) {
			l.up = false // no downlink possible; the uplink still works
			return
		}
		l.pipe = p
		return
	}
	if w, ok := rc.(io.Writer); ok {
		l.w = w
	} else {
		l.up = false
	}
}

func (l *Link) detach() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.w, l.pipe, l.up = nil, "", false
}
