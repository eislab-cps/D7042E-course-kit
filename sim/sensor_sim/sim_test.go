package main

import (
	"net"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// T2: every frame the simulator emits, for every scenario, with and without HMAC,
// is accepted by the shared FRAME_FORMAT parser with no dropped fields, carries
// exactly the scenario's keys, and follows the sequence rules (including the wrap).
func TestEveryEmittedFrameValidates(t *testing.T) {
	key, _ := frame.ParseKey(testKey)
	const n = 70000 // > 65536 so the sequence wraps
	for _, name := range ScenarioNames() {
		sc := Scenarios[name]
		want := map[string]bool{}
		for _, ch := range sc.Channels {
			want[ch.Key] = true
		}
		for _, k := range [][]byte{nil, key} {
			g := NewGenerator(sc, Config{ExcursionEvery: 60, ExcursionLen: 5}, 7)
			var tr frame.Tracker
			crossings := 0
			for i := 0; i < n; i++ {
				line := frame.Encode(g.Next(), k)
				if len(line) > frame.MaxLineLen {
					t.Fatalf("%s #%d: %d bytes", name, i, len(line))
				}
				f, err := frame.Parse(line)
				if err != nil {
					t.Fatalf("%s #%d %q: %v", name, i, line, err)
				}
				mode := frame.HMACOff
				if k != nil {
					mode = frame.HMACRequired
				}
				if err := frame.VerifyTag(f, mode, k); err != nil {
					t.Fatalf("%s #%d %q: %v", name, i, line, err)
				}
				c := frame.Check(f)
				if len(c.Range) != 0 || len(c.Unknown) != 0 {
					t.Fatalf("%s #%d %q: range %v unknown %v", name, i, line, c.Range, c.Unknown)
				}
				got := map[string]bool{}
				for _, fl := range c.Valid {
					if fl.Key != "BOOT" {
						got[fl.Key] = true
					}
				}
				if !sameKeys(got, want) {
					t.Fatalf("%s #%d %q: keys %v want %v", name, i, line, got, want)
				}
				if f.Boot() != (i == 0) {
					t.Fatalf("%s #%d: BOOT present=%v", name, i, f.Boot())
				}
				v := tr.Observe(f)
				if !v.Accept || v.Lost != 0 || (i > 0 && v.Reason != "in-order") {
					t.Fatalf("%s #%d: tracker %+v", name, i, v)
				}
				if raw, _ := f.Get(sc.Primary); float64(raw)/float64(frame.Keys[sc.Primary].Scale) > sc.Threshold {
					crossings++
				}
			}
			if crossings == 0 {
				t.Errorf("%s: primary %s never crossed %v", name, sc.Primary, sc.Threshold)
			}
		}
	}
}

// Without events the primary value stays below the threshold (no false alerts).
func TestNoEventsNoCrossings(t *testing.T) {
	for _, name := range ScenarioNames() {
		sc := Scenarios[name]
		g := NewGenerator(sc, Config{}, 3)
		for i := 0; i < 5000; i++ {
			f := g.Next()
			raw, _ := f.Get(sc.Primary)
			if float64(raw)/float64(frame.Keys[sc.Primary].Scale) > sc.Threshold {
				t.Fatalf("%s #%d: %d crossed without an event", name, i, raw)
			}
		}
	}
}

// A configured threshold is crossed by events and not in steady state.
func TestWithThreshold(t *testing.T) {
	sc := Scenarios["cold-chain"].WithThreshold(12)
	if Scenarios["cold-chain"].Threshold != 8 {
		t.Fatal("WithThreshold modified the shared scenario")
	}
	g := NewGenerator(sc, Config{ExcursionEvery: 50, ExcursionLen: 5}, 5)
	in, out := 0, 0
	for i := 0; i < 5000; i++ {
		raw, _ := g.Next().Get("TEMP")
		above := float64(raw)/100 > 12
		if g.InExcursion(i) && above {
			in++
		}
		if !g.InExcursion(i) && above {
			out++
		}
	}
	if in == 0 || out != 0 {
		t.Errorf("crossings in events %d, outside events %d", in, out)
	}
}

func TestTCPSinkDeliversFrames(t *testing.T) {
	s, err := listenTCP("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	conn, err := net.Dial("tcp", s.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Wait until the sink has registered the client.
	for i := 0; i < 100; i++ {
		s.mu.Lock()
		n := len(s.clients)
		s.mu.Unlock()
		if n == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	g := NewGenerator(Scenarios["press"], Config{}, 1)
	for i := 0; i < 10; i++ {
		if err := s.Write(frame.Encode(g.Next(), nil)); err != nil {
			t.Fatal(err)
		}
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	lr := frame.NewLineReader(conn, true)
	for i := 0; i < 10; i++ {
		l, err := lr.Next()
		if err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		f, err := frame.Parse(l)
		if err != nil || int(f.Seq) != i {
			t.Fatalf("line %d %q: seq %d err %v", i, l, f.Seq, err)
		}
	}
}

func TestOpenSinkRejectsUnknown(t *testing.T) {
	if _, err := OpenSink("udp://x"); err == nil || !strings.Contains(err.Error(), "-out") {
		t.Errorf("err %v", err)
	}
	if _, err := Lookup("nope"); err == nil {
		t.Error("unknown scenario accepted")
	}
}

func sameKeys(a, b map[string]bool) bool {
	var x, y []string
	for k := range a {
		x = append(x, k)
	}
	for k := range b {
		y = append(y, k)
	}
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, ",") == strings.Join(y, ",")
}
