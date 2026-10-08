package frame

import (
	"errors"
	"testing"
)

// The worked downlink frames of FRAME_FORMAT.md section 7, with the test key of section 6.
const testKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestDownlinkWorkedFrames(t *testing.T) {
	key, err := ParseKey(testKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		line string
		want error
		kind CommandKind
	}{
		{"C:0;BOOT:211;KA:1", nil, KindKeepAlive},
		{"C:12;CID:907;ACT:1", nil, KindAct},
		{"C:14;CID:908;SAFE:1", nil, KindSafe},
		{"C:15;ACT:1", ErrNoCID, 0},
		{"C:16;CID:909;ACT:1;SAFE:1", ErrActAndSafe, 0},
		{"C:17;CID:910;ACT:5", ErrActRange, 0},
		{"C:18;CID:911;ACT:1;LUX:3", nil, KindAct}, // unknown key ignored
	}
	for _, c := range cases {
		f, err := ParseDownlink(c.line)
		if err != nil {
			t.Fatalf("%s: parse: %v", c.line, err)
		}
		cmd, err := DecodeCommand(f, 1)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.line, err, c.want)
			continue
		}
		if err == nil && cmd.Kind != c.kind {
			t.Errorf("%s: kind %v, want %v", c.line, cmd.Kind, c.kind)
		}
	}
	// Tags from the section 7 table.
	good, _ := ParseDownlink("C:12;CID:907;ACT:1;H:354a9ceaa3cd5bba")
	if err := VerifyTag(good, HMACRequired, key); err != nil {
		t.Errorf("correct tag rejected: %v", err)
	}
	bad, _ := ParseDownlink("C:12;CID:907;ACT:0;H:354a9ceaa3cd5bba")
	if err := VerifyTag(bad, HMACRequired, key); !errors.Is(err, ErrBadTag) {
		t.Errorf("altered ACT accepted: %v", err)
	}
	if got := Tag(key, "C:12;CID:907;ACT:0;"); got != "bd6a536697f3341b" {
		t.Errorf("tag of the altered frame = %s, want bd6a536697f3341b", got)
	}
	if got := EncodeDownlink(Frame{Seq: 12, Fields: Command{Kind: KindAct, CID: 907, Act: 1}.Fields(0)}, key); got != "C:12;CID:907;ACT:1;H:354a9ceaa3cd5bba\n" {
		t.Errorf("EncodeDownlink = %q", got)
	}
}

// A downlink tag must not verify as an uplink frame and the other way round: the tag input
// starts with the sequence field, C: or S: (section 7).
func TestNoCrossDirectionReplay(t *testing.T) {
	key, _ := ParseKey(testKeyHex)
	up := Encode(Frame{Seq: 12, Fields: []Field{{"CID", 907}, {"ACT", 1}}}, key)
	tag := up[len(up)-17 : len(up)-1]
	f, err := ParseDownlink("C:12;CID:907;ACT:1;H:" + tag)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyTag(f, HMACRequired, key); !errors.Is(err, ErrBadTag) {
		t.Errorf("uplink tag accepted on a downlink frame: %v", err)
	}
}

func TestDownlinkSequenceUsesC(t *testing.T) {
	if _, err := ParseDownlink("S:1;KA:1"); !errors.Is(err, ErrNoSeq) {
		t.Errorf("S-prefixed line parsed as downlink: %v", err)
	}
	if _, err := Parse("C:1;TEMP:1"); !errors.Is(err, ErrNoSeq) {
		t.Errorf("C-prefixed line parsed as uplink: %v", err)
	}
	var tr Tracker
	f0, _ := ParseDownlink("C:5;KA:1")
	f1, _ := ParseDownlink("C:5;KA:1")
	if !tr.Observe(f0).Accept || tr.Observe(f1).Accept {
		t.Error("downlink duplicate not dropped by the section 4 rules")
	}
}

func TestCIDSetKeepsLastEight(t *testing.T) {
	var s CIDSet
	for i := int32(1); i <= 9; i++ {
		s.Add(i)
	}
	if s.Seen(1) {
		t.Error("CID 1 should be forgotten after nine adds")
	}
	for i := int32(2); i <= 9; i++ {
		if !s.Seen(i) {
			t.Errorf("CID %d forgotten", i)
		}
	}
}

func TestActuatorKeysInUplinkTable(t *testing.T) {
	f, err := Parse("S:431;TEMP:412;HUM:6020;ACTS:1;ACK:907;SAFE:0")
	if err != nil {
		t.Fatal(err)
	}
	c := Check(f)
	if len(c.Valid) != 5 || len(c.Unknown) != 0 || len(c.Range) != 0 {
		t.Errorf("check = %+v", c)
	}
}
