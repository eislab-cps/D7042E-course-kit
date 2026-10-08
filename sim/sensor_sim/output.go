package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

// Sink receives encoded frame lines.
type Sink interface {
	Write(line string) error
	Close() error
}

// OpenSink parses -out: "stdout", "pipe:/path/to/fifo" or "tcp://host:port".
func OpenSink(out string) (Sink, error) {
	switch {
	case out == "" || out == "stdout":
		return writerSink{os.Stdout}, nil
	case strings.HasPrefix(out, "pipe:"):
		return openPipe(strings.TrimPrefix(out, "pipe:"))
	case strings.HasPrefix(out, "tcp://"):
		return listenTCP(strings.TrimPrefix(out, "tcp://"))
	}
	return nil, fmt.Errorf("-out must be stdout, pipe:<path> or tcp://<host:port>, got %q", out)
}

type writerSink struct{ w io.Writer }

func (s writerSink) Write(line string) error { _, err := io.WriteString(s.w, line); return err }
func (s writerSink) Close() error            { return nil }

// openPipe creates the FIFO if needed and opens it for writing. Opening blocks until a
// reader (the gateway) opens the other end, so the stream starts at a frame boundary.
func openPipe(path string) (Sink, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			return nil, fmt.Errorf("mkfifo %s: %w", path, err)
		}
	}
	log.Printf("waiting for a reader on %s", path)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	return writerSink{f}, nil
}

// tcpSink listens on addr and writes every frame to all connected clients. A client
// that connects mid-stream starts at the next frame; one that fails is dropped.
type tcpSink struct {
	ln      net.Listener
	mu      sync.Mutex
	clients map[net.Conn]bool
	onLine  func(string) // downlink handler (FRAME_FORMAT.md section 7); nil: ignore input
}

// Downlink installs the handler for downlink lines read from connected clients.
func (s *tcpSink) Downlink(h func(string)) {
	s.mu.Lock()
	s.onLine = h
	s.mu.Unlock()
}

func listenTCP(addr string) (*tcpSink, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &tcpSink{ln: ln, clients: map[net.Conn]bool{}}
	log.Printf("serving frames on tcp://%s", ln.Addr())
	go s.accept()
	return s, nil
}

// Addr is the listening address (useful with port 0 in tests).
func (s *tcpSink) Addr() string { return s.ln.Addr().String() }

func (s *tcpSink) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.clients[c] = true
		h := s.onLine
		s.mu.Unlock()
		log.Printf("client connected: %s", c.RemoteAddr())
		if h != nil {
			go readLines(c, true, h)
		}
	}
}

func (s *tcpSink) Write(line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		if _, err := io.WriteString(c, line); err != nil {
			log.Printf("client dropped: %s: %v", c.RemoteAddr(), err)
			c.Close()
			delete(s.clients, c)
		}
	}
	return nil
}

func (s *tcpSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		c.Close()
	}
	return s.ln.Close()
}

// DownlinkSink is a sink that also carries downlink lines (TCP: the same connection).
type DownlinkSink interface {
	Downlink(func(string))
}

// readLines feeds every line of r to h until r ends (section 1 framing).
func readLines(r io.Reader, atBoundary bool, h func(string)) {
	lr := frame.NewLineReader(r, atBoundary)
	for {
		line, err := lr.Next()
		if err != nil {
			return
		}
		h(line)
	}
}

// OpenDownlink starts reading downlink lines from in ("stdin" or "pipe:<path>") and hands
// them to h. A pipe is created if needed and reopened whenever its writer goes away.
func OpenDownlink(in string, h func(string)) error {
	switch {
	case in == "stdin":
		go readLines(os.Stdin, true, h)
		return nil
	case strings.HasPrefix(in, "pipe:"):
		path := strings.TrimPrefix(in, "pipe:")
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				return fmt.Errorf("mkfifo %s: %w", path, err)
			}
		}
		go func() {
			for {
				f, err := os.Open(path) // blocks until the gateway opens it for writing
				if err != nil {
					log.Printf("downlink %s: %v", path, err)
					return
				}
				log.Printf("reading downlink from %s", path)
				readLines(f, true, h)
				f.Close()
			}
		}()
		return nil
	}
	return fmt.Errorf("-in must be stdin or pipe:<path>, got %q", in)
}
