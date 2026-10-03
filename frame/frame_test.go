package frame

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// testKey is the FRAME_FORMAT.md section 6 example key.
const testKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// T1: every plain worked frame in section 6 parses to the stated values.
func TestWorkedFrames(t *testing.T) {
	cases := []struct {
		line string
		seq  uint16
		want map[string]int32
	}{
		{"S:42;TEMP:2247;HUM:6130", 42, map[string]int32{"TEMP": 2247, "HUM": 6130}},
		{"S:1805;PWR:34215;OCC:1", 1805, map[string]int32{"PWR": 34215, "OCC": 1}},
		{"S:311;CO2:1184;TEMP:2362", 311, map[string]int32{"CO2": 1184, "TEMP": 2362}},
		{"S:65535;PRES:21750;CYC:48213", 65535, map[string]int32{"PRES": 21750, "CYC": 48213}},
		{"S:7;TEMP:-510;HUM:8800;BARO:10132", 7, map[string]int32{"TEMP": -510, "HUM": 8800, "BARO": 10132}},
		{"S:0;BOOT:40117;TEMP:2210;HUM:5980", 0, map[string]int32{"BOOT": 40117, "TEMP": 2210, "HUM": 5980}},
	}
	for _, c := range cases {
		f, err := Parse(c.line + "\n")
		if err != nil {
			t.Fatalf("%q: %v", c.line, err)
		}
		if f.Seq != c.seq {
			t.Errorf("%q: seq %d, want %d", c.line, f.Seq, c.seq)
		}
		for k, v := range c.want {
			if got, ok := f.Get(k); !ok || got != v {
				t.Errorf("%q: %s=%d,%v want %d", c.line, k, got, ok, v)
			}
		}
		chk := Check(f)
		if len(chk.Range) != 0 || len(chk.Unknown) != 0 {
			t.Errorf("%q: unexpected range/unknown %+v", c.line, chk)
		}
		if got := Encode(f, nil); got != c.line+"\n" {
			t.Errorf("round trip %q -> %q", c.line, got)
		}
	}
}

// T1: the section 6 error cases give the stated action.
func TestErrorCases(t *testing.T) {
	drops := map[string]error{
		"TEMP:2247;HUM:6130":                   ErrNoSeq,
		"S:45;TEMP:22.47":                      ErrSyntax,
		"S:46;TEMP:2247;TEMP:2250":             ErrDuplicateKey,
		"S:1;S:2;TEMP:1":                       ErrNoSeq,
		"S:70000;TEMP:1":                       ErrSyntax,
		"S:1":                                  ErrNoData,
		"S:1;temp:1":                           ErrSyntax,
		"S:1;TEMP:":                            ErrSyntax,
		"S:1;TEMP:1;H:ABCDEF":                  ErrSyntax,
		"S:1;H:0011223344556677;T:1":           ErrSyntax,
		"":                                     ErrEmpty,
		"S:1;TEMP:1\x01":                       ErrNonASCII,
		"S:1;TEMP:" + strings.Repeat("1", 200): ErrTooLong,
	}
	for line, want := range drops {
		if _, err := Parse(line); !errors.Is(err, want) {
			t.Errorf("%q: err %v, want %v", line, err, want)
		}
	}
	// Unknown key ignored, the rest kept.
	f, err := Parse("S:47;TEMP:2247;LUX:880")
	if err != nil {
		t.Fatal(err)
	}
	c := Check(f)
	if len(c.Valid) != 1 || c.Valid[0].Key != "TEMP" || len(c.Unknown) != 1 || c.Unknown[0] != "LUX" {
		t.Errorf("unknown key handling: %+v", c)
	}
	// Range error drops the field, not the frame.
	f, err = Parse("S:48;HUM:12000")
	if err != nil {
		t.Fatal(err)
	}
	if c := Check(f); len(c.Valid) != 0 || len(c.Range) != 1 {
		t.Errorf("range handling: %+v", c)
	}
	// CR before LF tolerated.
	if _, err := Parse("S:1;TEMP:1\r\n"); err != nil {
		t.Errorf("CRLF: %v", err)
	}
}

// T3: the section 6 HMAC tags recompute from the test key, and the verdicts hold.
func TestHMAC(t *testing.T) {
	key, err := ParseKey(testKey)
	if err != nil {
		t.Fatal(err)
	}
	for signed, tag := range map[string]string{
		"S:42;TEMP:2247;HUM:6130;": "6624d8fa493ef251",
		"S:43;TEMP:2251;HUM:6128;": "851759e6ae5ee27c",
		"S:43;TEMP:2951;HUM:6128;": "05552887c6a80e5c",
	} {
		if got := Tag(key, signed); got != tag {
			t.Errorf("Tag(%q) = %s, want %s", signed, got, tag)
		}
	}
	check := func(line string, mode HMACMode, want error) {
		t.Helper()
		f, err := Parse(line)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		if err := VerifyTag(f, mode, key); !errors.Is(err, want) {
			t.Errorf("%q mode %d: %v, want %v", line, mode, err, want)
		}
	}
	check("S:42;TEMP:2247;HUM:6130;H:6624d8fa493ef251", HMACRequired, nil)
	check("S:43;TEMP:2251;HUM:6128;H:851759e6ae5ee27c", HMACRequired, nil)
	check("S:43;TEMP:2951;HUM:6128;H:851759e6ae5ee27c", HMACRequired, ErrBadTag)
	check("S:44;TEMP:2249;HUM:6131", HMACRequired, ErrUntagged)
	check("S:44;TEMP:2249;HUM:6131", HMACOptional, nil)
	check("S:43;TEMP:2951;HUM:6128;H:851759e6ae5ee27c", HMACOptional, ErrBadTag)
	check("S:43;TEMP:2951;HUM:6128;H:851759e6ae5ee27c", HMACOff, nil)

	f := Frame{Seq: 42, Fields: []Field{{"TEMP", 2247}, {"HUM", 6130}}}
	if got := Encode(f, key); got != "S:42;TEMP:2247;HUM:6130;H:6624d8fa493ef251\n" {
		t.Errorf("Encode with key: %q", got)
	}
	if _, err := ParseKey("abcd"); err == nil {
		t.Error("short key accepted")
	}
	for _, s := range []string{"off", "optional", "required", ""} {
		if _, err := ParseHMACMode(s); err != nil {
			t.Errorf("mode %q: %v", s, err)
		}
	}
	if _, err := ParseHMACMode("yes"); err == nil {
		t.Error("bad mode accepted")
	}
}

// T4: sequence rules of section 4, including the replay and gap examples of section 6.
func TestTracker(t *testing.T) {
	var tr Tracker
	step := func(line string, accept bool, reason string, lost int) {
		t.Helper()
		f, err := Parse(line)
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		v := tr.Observe(f)
		if v.Accept != accept || v.Reason != reason || v.Lost != lost {
			t.Errorf("%q: %+v, want accept=%v reason=%s lost=%d", line, v, accept, reason, lost)
		}
	}
	step("S:42;TEMP:1", true, "first", 0)
	step("S:43;TEMP:1", true, "in-order", 0)
	step("S:42;TEMP:1", false, "duplicate", 0) // replay after 43
	step("S:43;TEMP:1", false, "duplicate", 0)
	step("S:48;TEMP:1", true, "gap", 4)
	step("S:52;TEMP:1", true, "gap", 3) // section 6: 52 after 48 -> 3 lost
	tr = Tracker{}
	step("S:65535;TEMP:1", true, "first", 0)
	step("S:0;TEMP:1", true, "in-order", 0) // wrap
	step("S:1;TEMP:1", true, "in-order", 0)
	step("S:40000;TEMP:1", false, "duplicate", 0) // d >= 32768 is stale
	step("S:0;BOOT:40117;TEMP:1", true, "restart", 0)
	step("S:1;TEMP:1", true, "in-order", 0)
	tr.Reset()
	step("S:900;TEMP:1", true, "first", 0)
}

func TestLineReader(t *testing.T) {
	long := strings.Repeat("X", 200)
	in := "EMP:22;HUM:1\nS:1;TEMP:1\n" + long + "\nS:2;TEMP:2\r\nS:3;TEM"
	lr := NewLineReader(strings.NewReader(in), false)
	var got []string
	for {
		l, err := lr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, l)
	}
	want := []string{"S:1;TEMP:1", "S:2;TEMP:2\r"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("lines %q, want %q", got, want)
	}
	if lr.Discarded != 3 { // partial start, too long, trailing fragment
		t.Errorf("discarded %d, want 3", lr.Discarded)
	}
	lr = NewLineReader(strings.NewReader("S:1;TEMP:1\n"), true)
	if l, err := lr.Next(); err != nil || l != "S:1;TEMP:1" {
		t.Errorf("atBoundary: %q %v", l, err)
	}
}
