package device

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// T5: SERIAL_SOURCE selects pipe, TCP and serial device.
func TestParseSource(t *testing.T) {
	ok := map[string]Source{
		"":                     {"pipe", DefaultPipe},
		"pipe":                 {"pipe", DefaultPipe},
		"pipe:/tmp/x":          {"pipe", "/tmp/x"},
		"tcp://localhost:7000": {"tcp", "localhost:7000"},
		"/dev/ttyUSB0":         {"serial", "/dev/ttyUSB0"},
		"/dev/ttyACM0":         {"serial", "/dev/ttyACM0"},
	}
	for spec, want := range ok {
		got, err := ParseSource(spec, "")
		if err != nil || got != want {
			t.Errorf("%q: %+v %v, want %+v", spec, got, err, want)
		}
	}
	if got, _ := ParseSource("pipe", "/run/pico"); got.Addr != "/run/pico" {
		t.Errorf("FRAME_PIPE not used: %+v", got)
	}
	for _, bad := range []string{"udp://x:1", "tcp://nohost", "COM3", "/tmp/file"} {
		if _, err := ParseSource(bad, ""); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func readAll(t *testing.T, src Source, n int) []string {
	t.Helper()
	rc, atBoundary, err := src.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	lr := frame.NewLineReader(rc, atBoundary)
	var out []string
	for len(out) < n {
		l, err := lr.Next()
		if err != nil {
			t.Fatalf("after %d lines: %v", len(out), err)
		}
		out = append(out, l)
	}
	return out
}

func TestOpenPipe(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pico")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	go func() {
		w, err := os.OpenFile(p, os.O_WRONLY, 0)
		if err != nil {
			return
		}
		w.WriteString("S:0;BOOT:9;TEMP:400\nS:1;TEMP:401\n")
		w.Close()
	}()
	src, _ := ParseSource("pipe:"+p, "")
	got := readAll(t, src, 2)
	if got[0] != "S:0;BOOT:9;TEMP:400" || got[1] != "S:1;TEMP:401" {
		t.Errorf("pipe lines %q", got)
	}
}

func TestOpenTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.Write([]byte("S:5;PRES:20000;CYC:5\n"))
		c.Close()
	}()
	src, _ := ParseSource("tcp://"+ln.Addr().String(), "")
	if got := readAll(t, src, 1); got[0] != "S:5;PRES:20000;CYC:5" {
		t.Errorf("tcp line %q", got)
	}
}

// A serial device may be opened mid-frame, so its first line is discarded.
func TestOpenSerialDiscardsFirstLine(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tty")
	os.WriteFile(p, []byte("MP:400\nS:2;TEMP:402\n"), 0o600)
	got := readAll(t, Source{Kind: "serial", Addr: p}, 1)
	if got[0] != "S:2;TEMP:402" {
		t.Errorf("serial line %q", got)
	}
}

// Parsing: the ingest applies every FRAME_FORMAT rule and counts what it drops.
func TestIngestCounters(t *testing.T) {
	key, _ := frame.ParseKey(testKey)
	in := NewIngest(frame.HMACOff, nil)
	for _, l := range []string{
		"S:0;BOOT:40117;TEMP:2210;HUM:5980", // restart
		"S:1;TEMP:2247;HUM:6130",            // in order
		"S:1;TEMP:2247;HUM:6130",            // duplicate
		"S:5;TEMP:2250;HUM:12000",           // gap of 3; HUM out of range
		"TEMP:2247",                         // malformed
		"S:6;TEMP:2251;LUX:9",               // unknown key ignored
	} {
		in.Line(l)
	}
	s := in.Stats()
	want := Stats{Accepted: 4, Lost: 3, Duplicate: 1, Malformed: 1, Range: 1, Restarts: 1}
	if s != want {
		t.Errorf("stats %+v, want %+v", s, want)
	}
	r, ok := in.Latest("TEMP")
	if !ok || r.Raw != 2251 || r.Value != 22.51 || r.Unit != "°C" || r.Seq != 6 {
		t.Errorf("TEMP %+v", r)
	}
	if h, _ := in.Latest("HUM"); h.Raw != 6130 {
		t.Errorf("HUM kept the out-of-range value: %+v", h)
	}

	in = NewIngest(frame.HMACRequired, key)
	in.Line("S:42;TEMP:2247;HUM:6130;H:6624d8fa493ef251")
	in.Line("S:43;TEMP:2951;HUM:6128;H:851759e6ae5ee27c") // tampered
	in.Line("S:44;TEMP:2249;HUM:6131")                    // untagged
	if s := in.Stats(); s.Accepted != 1 || s.HMACFail != 2 {
		t.Errorf("hmac stats %+v", s)
	}
}

// Run reads from a TCP source and reconnects when it closes; after reopening, the
// first frame is treated as "first frame seen" rather than stale.
func TestRunReconnects(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for _, batch := range []string{"S:10;TEMP:400\nS:11;TEMP:401\n", "S:3;TEMP:405\n"} {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Write([]byte(batch))
			c.Close()
		}
	}()
	in := NewIngest(frame.HMACOff, nil)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() { Run(Source{Kind: "tcp", Addr: ln.Addr().String()}, in, stop); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for in.Stats().Accepted < 3 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	close(stop)
	<-done
	if s := in.Stats(); s.Accepted != 3 || s.Duplicate != 0 {
		t.Errorf("stats %+v", s)
	}
	if r, _ := in.Latest("TEMP"); r.Raw != 405 {
		t.Errorf("latest %+v", r)
	}
}
