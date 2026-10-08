// Package device is the gateway's device side: it opens SERIAL_SOURCE and applies the
// FRAME_FORMAT.md rules (via frame/) to every line, keeping counters and the latest
// reading per key, and it offers the write side of the same link for downlink frames
// (Link, FRAME_FORMAT.md section 7). It contains no Arrowhead calls.
package device

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
)

// Source is where the gateway reads device frames from (SERIAL_SOURCE).
type Source struct {
	Kind string // "pipe", "tcp" or "serial"
	Addr string // file path or host:port
}

// DefaultPipe is the named pipe used by SERIAL_SOURCE=pipe (the simulator's
// -out pipe:/tmp/pico).
const DefaultPipe = "/tmp/pico"

// ParseSource interprets SERIAL_SOURCE:
//
//	pipe                -> named pipe DefaultPipe (or FRAME_PIPE)
//	pipe:/path/to/fifo  -> that named pipe
//	tcp://host:port     -> the simulator's TCP socket
//	/dev/ttyUSB0        -> a serial device (any path under /dev/)
func ParseSource(spec, framePipe string) (Source, error) {
	switch {
	case spec == "" || spec == "pipe":
		if framePipe == "" {
			framePipe = DefaultPipe
		}
		return Source{Kind: "pipe", Addr: framePipe}, nil
	case strings.HasPrefix(spec, "pipe:"):
		return Source{Kind: "pipe", Addr: strings.TrimPrefix(spec, "pipe:")}, nil
	case strings.HasPrefix(spec, "tcp://"):
		addr := strings.TrimPrefix(spec, "tcp://")
		if _, _, err := net.SplitHostPort(addr); err != nil {
			return Source{}, fmt.Errorf("SERIAL_SOURCE %q: %w", spec, err)
		}
		return Source{Kind: "tcp", Addr: addr}, nil
	case strings.HasPrefix(spec, "/dev/"):
		return Source{Kind: "serial", Addr: spec}, nil
	}
	return Source{}, fmt.Errorf("SERIAL_SOURCE must be pipe, pipe:<path>, tcp://<host:port> or /dev/<device>, got %q", spec)
}

// Open connects to the source. atBoundary reports whether the stream is known to
// start at a frame boundary: the simulator writes whole lines to a pipe reader and
// to a new TCP client, while a serial line may be opened mid-frame. A serial device
// and a TCP connection are opened for reading and writing, so rc is also an io.Writer
// for downlink frames; a named pipe carries the uplink only (see DownlinkPipe).
func (s Source) Open() (rc io.ReadCloser, atBoundary bool, err error) {
	switch s.Kind {
	case "pipe":
		// Opening a FIFO for reading blocks until the simulator opens it for writing.
		f, err := os.Open(s.Addr)
		return f, true, err
	case "tcp":
		c, err := net.Dial("tcp", s.Addr)
		return c, true, err
	case "serial":
		// The line speed must be set beforehand, e.g. `stty -F /dev/ttyUSB0 115200 raw`.
		// Read-write for downlink frames; O_NOCTTY so the device never becomes the
		// gateway's controlling terminal.
		f, err := os.OpenFile(s.Addr, os.O_RDWR|syscall.O_NOCTTY, 0)
		return f, false, err
	}
	return nil, false, fmt.Errorf("unknown source kind %q", s.Kind)
}

// DownlinkPipe is the named pipe that carries downlink frames when the uplink is a
// named pipe: "<pipe>.down" (FRAME_FORMAT.md section 7). Empty for other kinds.
func (s Source) DownlinkPipe() string {
	if s.Kind != "pipe" {
		return ""
	}
	return s.Addr + ".down"
}

func (s Source) String() string {
	if s.Kind == "tcp" {
		return "tcp://" + s.Addr
	}
	return s.Kind + ":" + s.Addr
}
