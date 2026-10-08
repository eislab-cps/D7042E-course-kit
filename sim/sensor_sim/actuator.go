package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

// Actuator is the device side of FRAME_FORMAT.md section 7: it applies downlink commands
// once per CID, reports ACTS, ACK and SAFE, and enters its safe state on SAFE:1 or when no
// valid downlink arrives for the link timeout. It starts in its safe state.
type Actuator struct {
	Kind        string        // "cooling" or "valve"
	SafeValue   int32         // cooling: 1 (on), valve: 0 (closed)
	Delay       time.Duration // time before ACTS follows ACT
	Stuck       bool          // fault: acknowledge commands but never move
	LinkTimeout time.Duration

	mode frame.HMACMode
	key  []byte
	now  func() time.Time

	mu        sync.Mutex
	observed  int32
	ack       int32
	safe      bool
	pending   bool
	pendingTo int32
	dueAt     time.Time
	lastValid time.Time
	tracker   frame.Tracker
	cids      frame.CIDSet
	Stats     ActuatorStats
}

// ActuatorStats counts downlink outcomes.
type ActuatorStats struct {
	Accepted, Applied, Repeated, Malformed, Invalid, HMACFail, Duplicate, Timeouts int
}

// NewActuator returns an actuator of kind in its safe state. With requireTags, untagged
// downlink frames are dropped; otherwise a tag is verified when present and a key is set.
func NewActuator(kind string, delay, linkTimeout time.Duration, stuck bool, key []byte, requireTags bool, now func() time.Time) (*Actuator, error) {
	a := &Actuator{Kind: kind, Delay: delay, Stuck: stuck, LinkTimeout: linkTimeout, key: key, now: now}
	switch kind {
	case "cooling":
		a.SafeValue = 1
	case "valve":
		a.SafeValue = 0
	default:
		return nil, fmt.Errorf("-actuator must be cooling or valve, got %q", kind)
	}
	switch {
	case requireTags:
		a.mode = frame.HMACRequired
	case key != nil:
		a.mode = frame.HMACOptional
	default:
		a.mode = frame.HMACOff
	}
	a.observed, a.safe, a.lastValid = a.SafeValue, true, now()
	return a, nil
}

// Downlink handles one downlink line (without LF).
func (a *Actuator) Downlink(line string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f, err := frame.ParseDownlink(line)
	if err != nil {
		a.Stats.Malformed++
		log.Printf("downlink dropped: %v: %q", err, line)
		return
	}
	if err := frame.VerifyTag(f, a.mode, a.key); err != nil {
		a.Stats.HMACFail++
		log.Printf("downlink hmac failure C=%d: %v", f.Seq, err)
		return
	}
	cmd, err := frame.DecodeCommand(f, 1)
	if err != nil {
		a.Stats.Invalid++
		log.Printf("downlink dropped: %v: %q", err, line)
		return
	}
	if v := a.tracker.Observe(f); !v.Accept {
		a.Stats.Duplicate++
		log.Printf("downlink dropped: duplicate or stale C=%d", f.Seq)
		return
	}
	a.Stats.Accepted++
	a.lastValid = a.now()
	if cmd.Boot {
		log.Printf("gateway restart seen in the downlink (C=%d)", f.Seq)
	}
	if cmd.Kind == frame.KindKeepAlive {
		return
	}
	if a.cids.Seen(cmd.CID) {
		a.Stats.Repeated++
		log.Printf("downlink CID %d already applied; nothing done", cmd.CID)
		return
	}
	a.cids.Add(cmd.CID)
	a.ack = cmd.CID
	a.Stats.Applied++
	switch cmd.Kind {
	case frame.KindSafe:
		a.enterSafe(fmt.Sprintf("SAFE command (CID %d)", cmd.CID))
	case frame.KindAct:
		a.safe = false
		if a.Stuck {
			log.Printf("CID %d: ACT:%d acknowledged, actuator stuck at %d", cmd.CID, cmd.Act, a.observed)
			return
		}
		a.pending, a.pendingTo, a.dueAt = true, cmd.Act, a.now().Add(a.Delay)
		log.Printf("CID %d: ACT:%d, observed in %v", cmd.CID, cmd.Act, a.Delay)
	}
}

// enterSafe puts the actuator in its safe state. Called with mu held.
func (a *Actuator) enterSafe(why string) {
	a.safe, a.pending = true, false
	if !a.Stuck {
		a.observed = a.SafeValue
	}
	log.Printf("safe state: %s; %s=%d", why, a.Kind, a.observed)
}

// Tick applies a due command and the link timeout; call it once per sample.
func (a *Actuator) Tick() {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	if a.pending && !now.Before(a.dueAt) {
		a.observed, a.pending = a.pendingTo, false
	}
	if a.LinkTimeout > 0 && !a.safe && now.Sub(a.lastValid) > a.LinkTimeout {
		a.Stats.Timeouts++
		a.enterSafe(fmt.Sprintf("no valid downlink for %v", a.LinkTimeout))
	}
}

// Fields are the uplink feedback fields: ACTS, ACK, SAFE.
func (a *Actuator) Fields() []frame.Field {
	a.mu.Lock()
	defer a.mu.Unlock()
	safe := int32(0)
	if a.safe {
		safe = 1
	}
	return []frame.Field{{Key: "ACTS", Value: a.observed}, {Key: "ACK", Value: a.ack}, {Key: "SAFE", Value: safe}}
}

// Observed returns the observed actuator state.
func (a *Actuator) Observed() int32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.observed
}
