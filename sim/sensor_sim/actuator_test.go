package main

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }
func fields(a *Actuator) map[string]int32 {
	m := map[string]int32{}
	for _, f := range a.Fields() {
		m[f.Key] = f.Value
	}
	return m
}

func newTestActuator(t *testing.T, stuck bool, key []byte, require bool) (*Actuator, *clock) {
	t.Helper()
	c := &clock{t: time.Unix(1000, 0)}
	a, err := NewActuator("cooling", 500*time.Millisecond, 10*time.Second, stuck, key, require, c.now)
	if err != nil {
		t.Fatal(err)
	}
	return a, c
}

func TestActuatorStartsSafeAndAppliesOncePerCID(t *testing.T) {
	a, c := newTestActuator(t, false, nil, false)
	if f := fields(a); f["ACTS"] != 1 || f["SAFE"] != 1 || f["ACK"] != 0 {
		t.Fatalf("start state %v, want cooling on, safe, no ACK", f)
	}
	a.Downlink("C:1;CID:907;ACT:0")
	a.Tick()
	if f := fields(a); f["ACTS"] != 1 || f["ACK"] != 907 || f["SAFE"] != 0 {
		t.Fatalf("before the delay: %v", f)
	}
	c.add(600 * time.Millisecond)
	a.Tick()
	if f := fields(a); f["ACTS"] != 0 {
		t.Fatalf("after the delay ACTS = %d, want 0", f["ACTS"])
	}
	a.Downlink("C:2;CID:907;ACT:1") // repeated CID: not applied
	c.add(time.Second)
	a.Tick()
	if f := fields(a); f["ACTS"] != 0 || a.Stats.Repeated != 1 {
		t.Fatalf("repeated CID applied: %v, stats %+v", f, a.Stats)
	}
}

func TestActuatorSafeCommandAndLinkTimeout(t *testing.T) {
	a, c := newTestActuator(t, false, nil, false)
	a.Downlink("C:1;CID:1;ACT:0")
	c.add(time.Second)
	a.Tick()
	a.Downlink("C:2;CID:2;SAFE:1")
	if f := fields(a); f["ACTS"] != 1 || f["SAFE"] != 1 || f["ACK"] != 2 {
		t.Fatalf("after SAFE: %v", f)
	}
	a.Downlink("C:3;CID:3;ACT:0") // a new command leaves the safe state
	c.add(time.Second)
	a.Tick()
	if f := fields(a); f["ACTS"] != 0 || f["SAFE"] != 0 {
		t.Fatalf("after a new command: %v", f)
	}
	c.add(9 * time.Second)
	a.Downlink("C:4;KA:1") // keep-alive keeps the link alive
	c.add(9 * time.Second)
	a.Tick()
	if fields(a)["SAFE"] != 0 {
		t.Fatal("safe state despite keep-alives")
	}
	c.add(2 * time.Second)
	a.Tick()
	if f := fields(a); f["SAFE"] != 1 || f["ACTS"] != 1 || a.Stats.Timeouts != 1 {
		t.Fatalf("no safe state after the link timeout: %v %+v", f, a.Stats)
	}
}

func TestActuatorStuckFault(t *testing.T) {
	a, c := newTestActuator(t, true, nil, false)
	a.Downlink("C:1;CID:5;ACT:0")
	c.add(2 * time.Second)
	a.Tick()
	if f := fields(a); f["ACTS"] != 1 || f["ACK"] != 5 {
		t.Fatalf("stuck actuator: %v, want ACTS 1 (unmoved) and ACK 5", f)
	}
}

func TestActuatorDropsInvalidAndStaleFrames(t *testing.T) {
	a, _ := newTestActuator(t, false, nil, false)
	for _, l := range []string{"C:1;ACT:0", "C:2;CID:9;ACT:1;SAFE:1", "C:3;CID:9;ACT:4", "S:4;CID:9;ACT:0"} {
		a.Downlink(l)
	}
	a.Downlink("C:10;CID:20;ACT:0")
	a.Downlink("C:10;CID:21;ACT:1") // stale sequence
	if a.Stats.Invalid != 3 || a.Stats.Malformed != 1 || a.Stats.Duplicate != 1 || a.Stats.Applied != 1 {
		t.Fatalf("stats %+v", a.Stats)
	}
}

func TestActuatorRequiresTagsWithHMAC(t *testing.T) {
	key, _ := frame.ParseKey("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	a, _ := newTestActuator(t, false, key, true)
	a.Downlink("C:12;CID:907;ACT:1")                    // untagged: dropped
	a.Downlink("C:12;CID:907;ACT:0;H:354a9ceaa3cd5bba") // altered: dropped
	a.Downlink("C:12;CID:907;ACT:1;H:354a9ceaa3cd5bba") // correct: applied
	if a.Stats.HMACFail != 2 || a.Stats.Applied != 1 {
		t.Fatalf("stats %+v", a.Stats)
	}
}

// End to end over TCP: the gateway side connects, reads uplink, writes downlink on the
// same connection.
func TestTCPSinkCarriesDownlink(t *testing.T) {
	s, err := listenTCP("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got := make(chan string, 1)
	s.Downlink(func(l string) { got <- l })
	c, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("C:0;BOOT:1;KA:1\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case l := <-got:
		if l != "C:0;BOOT:1;KA:1" {
			t.Fatalf("got %q", l)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no downlink line received")
	}
	time.Sleep(50 * time.Millisecond) // let accept register the client for writing
	if err := s.Write("S:0;TEMP:1\n"); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "S:0;TEMP:1" {
		t.Fatalf("uplink %q %v", line, err)
	}
}
