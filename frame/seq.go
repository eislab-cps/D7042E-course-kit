package frame

// Verdict is the Tracker's decision for one frame (section 4).
type Verdict struct {
	Accept  bool
	Lost    int    // frames missing before this one (gap case)
	Restart bool   // frame carried BOOT; sequence tracking was reset
	Reason  string // "in-order", "gap", "duplicate", "restart", "first"
}

// Tracker applies the section 4 sequence rules. The zero value is ready to use and
// treats the next frame as "first frame seen".
type Tracker struct {
	last    uint16
	started bool
}

// Reset makes the next frame "first frame seen", e.g. after the source reopened.
func (t *Tracker) Reset() { t.started = false }

// Observe decides on f and updates the state when f is accepted.
func (t *Tracker) Observe(f Frame) Verdict {
	switch {
	case f.Boot():
		t.last, t.started = f.Seq, true
		return Verdict{Accept: true, Restart: true, Reason: "restart"}
	case !t.started:
		t.last, t.started = f.Seq, true
		return Verdict{Accept: true, Reason: "first"}
	}
	d := int(uint16(f.Seq - t.last))
	switch {
	case d == 1:
		t.last = f.Seq
		return Verdict{Accept: true, Reason: "in-order"}
	case d >= 2 && d < 32768:
		t.last = f.Seq
		return Verdict{Accept: true, Lost: d - 1, Reason: "gap"}
	default:
		return Verdict{Accept: false, Reason: "duplicate"}
	}
}
