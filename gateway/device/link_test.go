package device

import (
	"bufio"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

// waitFor polls cond for up to 5 s.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLinkDetachedAndOneLine(t *testing.T) {
	var l Link
	if l.Up() {
		t.Error("new link up")
	}
	if err := l.WriteLine("C:0;BOOT:1;KA:1"); !errors.Is(err, ErrNoLink) {
		t.Errorf("detached write: %v", err)
	}
	l.attach(Source{Kind: "tcp"}, nopConn{})
	if err := l.WriteLine("C:0;KA:1\nC:1;KA:1"); err == nil {
		t.Error("two lines accepted")
	}
}

type nopConn struct{}

func (nopConn) Read([]byte) (int, error)    { return 0, nil }
func (nopConn) Write(b []byte) (int, error) { return len(b), nil }
func (nopConn) Close() error                { return nil }

// R8: the downlink goes back over the TCP connection the frames arrive on, and the link
// is down again once the connection closes.
func TestLinkTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan string, 1)
	closeConn := make(chan struct{})
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c.Write([]byte("S:0;BOOT:9;TEMP:400\n"))
		l, _ := bufio.NewReader(c).ReadString('\n')
		got <- l
		<-closeConn
		c.Close()
	}()
	in := NewIngest(frame.HMACOff, nil)
	var link Link
	stop := make(chan struct{})
	defer close(stop)
	go RunLink(Source{Kind: "tcp", Addr: ln.Addr().String()}, in, &link, stop)
	waitFor(t, "first frame", func() bool { return in.Stats().Accepted == 1 })
	if err := link.WriteLine("C:0;BOOT:7;CID:1;ACT:1"); err != nil {
		t.Fatal(err)
	}
	if l := <-got; l != "C:0;BOOT:7;CID:1;ACT:1\n" {
		t.Errorf("device read %q", l)
	}
	close(closeConn)
	waitFor(t, "link down", func() bool { return !link.Up() })
	if err := link.WriteLine("C:1;KA:1"); !errors.Is(err, ErrNoLink) {
		t.Errorf("write after close: %v", err)
	}
}

// R8 in pipe mode: the downlink goes to <pipe>.down; with no reader the write fails with
// ErrNoLink instead of blocking, and nothing is buffered for a later reader.
func TestLinkPipe(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pico")
	if err := syscall.Mkfifo(p, 0o600); err != nil {
		t.Skip("mkfifo:", err)
	}
	up, err := os.OpenFile(p, os.O_RDWR, 0) // the simulator's write end, held open
	if err != nil {
		t.Fatal(err)
	}
	defer up.Close()
	up.WriteString("S:0;BOOT:9;TEMP:400\n")
	in := NewIngest(frame.HMACOff, nil)
	var link Link
	stop := make(chan struct{})
	defer close(stop)
	go RunLink(Source{Kind: "pipe", Addr: p}, in, &link, stop)
	waitFor(t, "first frame", func() bool { return in.Stats().Accepted == 1 })

	if err := link.WriteLine("C:0;BOOT:7;CID:1;ACT:1"); !errors.Is(err, ErrNoLink) {
		t.Fatalf("write without a downlink reader: %v", err)
	}
	down, err := os.OpenFile(p+".down", os.O_RDONLY|syscall.O_NONBLOCK, 0) // the simulator's read end
	if err != nil {
		t.Fatal(err)
	}
	defer down.Close()
	if err := link.WriteLine("C:1;CID:2;ACT:0"); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	var n int
	waitFor(t, "downlink line", func() bool { n, _ = down.Read(buf); return n > 0 })
	if got := string(buf[:n]); got != "C:1;CID:2;ACT:0\n" {
		t.Errorf("device read %q (a stale frame was buffered if it contains CID:1)", got)
	}
}

// Correction 3 of the R8 review: the gateway writes downlink frames to a real serial
// device. A pseudo-terminal goes through the kernel's tty layer like /dev/ttyUSB0; the
// test plays the device on the master side and sets the line raw, as `stty raw` would.
func TestLinkSerialTTY(t *testing.T) {
	master, slavePath, err := openPTY()
	if err != nil {
		t.Skip("no pseudo-terminal:", err)
	}
	defer master.Close()
	slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	if err := makeRaw(slave); err != nil {
		t.Fatal(err)
	}
	src, err := ParseSource(slavePath, "")
	if err != nil || src.Kind != "serial" {
		t.Fatalf("%q: %+v %v", slavePath, src, err)
	}
	in := NewIngest(frame.HMACOff, nil)
	var link Link
	stop := make(chan struct{})
	defer close(stop)
	go RunLink(src, in, &link, stop)
	waitFor(t, "link up", link.Up)
	// The first line on a serial device is discarded (it may be partial).
	master.WriteString("P:400\nS:0;BOOT:9;TEMP:400;ACTS:1;ACK:0;SAFE:1\n")
	waitFor(t, "first frame", func() bool { return in.Stats().Accepted == 1 })
	if r, _ := in.Latest("SAFE"); r.Raw != 1 {
		t.Errorf("SAFE %+v", r)
	}

	key, _ := frame.ParseKey(testKey)
	line := frame.EncodeDownlink(frame.Frame{Seq: 12, Fields: []frame.Field{{Key: "CID", Value: 907}, {Key: "ACT", Value: 1}}}, key)
	if err := link.WriteLine(line); err != nil {
		t.Fatal(err)
	}
	got, err := bufio.NewReader(master).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if want := "C:12;CID:907;ACT:1;H:354a9ceaa3cd5bba\n"; got != want {
		t.Errorf("device read %q, want %q", got, want)
	}
	if !strings.HasSuffix(got, "354a9ceaa3cd5bba\n") {
		t.Error("tag differs from FRAME_FORMAT.md section 7")
	}
}
