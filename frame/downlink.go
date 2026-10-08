package frame

import "errors"

// CommandKind is the kind of a downlink frame (section 7).
type CommandKind int

const (
	KindKeepAlive CommandKind = iota // KA:1, the gateway is alive
	KindAct                          // CID and ACT: set the actuator
	KindSafe                         // CID and SAFE:1: enter the safe state
)

// Command is a validated downlink frame.
type Command struct {
	Kind CommandKind
	CID  int32 // 0 for a keep-alive
	Act  int32 // the commanded state, for KindAct
	Boot bool  // the frame carried the gateway's restart marker
}

// Errors returned by DecodeCommand. Every one means "drop the whole frame" (section 7).
var (
	ErrNotCommand = errors.New("frame: downlink has no ACT, SAFE or KA")
	ErrNoCID      = errors.New("frame: command without CID")
	ErrActAndSafe = errors.New("frame: both ACT and SAFE in one frame")
	ErrActRange   = errors.New("frame: ACT outside the actuator's range")
	ErrBadCID     = errors.New("frame: CID outside 1..2147483647")
)

// DecodeCommand applies the section 7 validity rules to a parsed downlink frame. actMax
// is the actuator's largest ACT value (1 for cooling and valve). Unknown keys are ignored.
func DecodeCommand(f Frame, actMax int32) (Command, error) {
	cid, hasCID := f.Get("CID")
	act, hasAct := f.Get("ACT")
	safe, hasSafe := f.Get("SAFE")
	_, hasKA := f.Get("KA")
	c := Command{Boot: f.Boot()}
	switch {
	case hasAct && hasSafe:
		return Command{}, ErrActAndSafe
	case hasAct || hasSafe:
		if !hasCID {
			return Command{}, ErrNoCID
		}
		if cid < 1 {
			return Command{}, ErrBadCID
		}
		c.CID = cid
		if hasSafe {
			if safe != 1 {
				return Command{}, ErrSyntax
			}
			c.Kind = KindSafe
			return c, nil
		}
		if act < 0 || act > actMax {
			return Command{}, ErrActRange
		}
		c.Kind, c.Act = KindAct, act
		return c, nil
	case hasKA:
		c.Kind = KindKeepAlive
		return c, nil
	}
	return Command{}, ErrNotCommand
}

// Fields returns the downlink data fields for c, in wire order (BOOT first if set).
func (c Command) Fields(bootEpoch int32) []Field {
	var fl []Field
	if c.Boot {
		fl = append(fl, Field{Key: "BOOT", Value: bootEpoch})
	}
	switch c.Kind {
	case KindAct:
		fl = append(fl, Field{Key: "CID", Value: c.CID}, Field{Key: "ACT", Value: c.Act})
	case KindSafe:
		fl = append(fl, Field{Key: "CID", Value: c.CID}, Field{Key: "SAFE", Value: 1})
	default:
		fl = append(fl, Field{Key: "KA", Value: 1})
	}
	return fl
}

// CIDSet remembers the last 8 applied command IDs (section 7, idempotency).
type CIDSet struct {
	ids  [8]int32
	n    int
	next int
}

// Seen reports whether id is among the remembered IDs.
func (s *CIDSet) Seen(id int32) bool {
	for i := 0; i < s.n; i++ {
		if s.ids[i] == id {
			return true
		}
	}
	return false
}

// Add remembers id, forgetting the oldest when full.
func (s *CIDSet) Add(id int32) {
	s.ids[s.next] = id
	s.next = (s.next + 1) % len(s.ids)
	if s.n < len(s.ids) {
		s.n++
	}
}
