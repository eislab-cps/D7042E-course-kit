package device

import (
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

// Reading is the latest accepted value of one key, already scaled to physical units.
type Reading struct {
	Key   string
	Raw   int32
	Value float64
	Unit  string
	Seq   uint16
	At    time.Time
}

// Stats are the FRAME_FORMAT.md section 4 counters.
type Stats struct {
	Accepted, Lost, Duplicate, Malformed, Range, HMACFail, Restarts, Discarded int
}

// Ingest applies the frame contract to lines and keeps the latest readings.
type Ingest struct {
	HMACMode frame.HMACMode
	HMACKey  []byte

	mu      sync.Mutex
	tracker frame.Tracker
	latest  map[string]Reading
	stats   Stats
	now     func() time.Time
}

func NewIngest(mode frame.HMACMode, key []byte) *Ingest {
	return &Ingest{HMACMode: mode, HMACKey: key, latest: map[string]Reading{}, now: time.Now}
}

// Line processes one line (without LF) and reports whether the frame was accepted.
func (in *Ingest) Line(line string) bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	f, err := frame.Parse(line)
	if err != nil {
		in.stats.Malformed++
		log.Printf("frame dropped: %v: %q", err, line)
		return false
	}
	if err := frame.VerifyTag(f, in.HMACMode, in.HMACKey); err != nil {
		in.stats.HMACFail++
		log.Printf("hmac failure seq=%d: %v", f.Seq, err)
		return false
	}
	v := in.tracker.Observe(f)
	if !v.Accept {
		in.stats.Duplicate++
		log.Printf("frame dropped: duplicate or stale seq=%d", f.Seq)
		return false
	}
	in.stats.Accepted++
	in.stats.Lost += v.Lost
	if v.Lost > 0 {
		log.Printf("sequence gap before seq=%d: %d frame(s) lost", f.Seq, v.Lost)
	}
	if v.Restart {
		in.stats.Restarts++
		b, _ := f.Get("BOOT")
		log.Printf("device restart, boot epoch %d", b)
	}
	c := frame.Check(f)
	in.stats.Range += len(c.Range)
	for _, fl := range c.Range {
		log.Printf("range error seq=%d %s=%d dropped", f.Seq, fl.Key, fl.Value)
	}
	at := in.now()
	for _, fl := range c.Valid {
		if fl.Key == "BOOT" {
			continue
		}
		spec := frame.Keys[fl.Key]
		in.latest[fl.Key] = Reading{
			Key: fl.Key, Raw: fl.Value, Value: float64(fl.Value) / float64(spec.Scale),
			Unit: spec.Unit, Seq: f.Seq, At: at,
		}
	}
	return true
}

// Latest returns the most recent reading for key.
func (in *Ingest) Latest(key string) (Reading, bool) {
	in.mu.Lock()
	defer in.mu.Unlock()
	r, ok := in.latest[key]
	return r, ok
}

// Stats returns a copy of the counters.
func (in *Ingest) Stats() Stats {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.stats
}

// SourceClosed resets sequence tracking: the next frame is "first frame seen".
func (in *Ingest) SourceClosed(discarded int) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.tracker.Reset()
	in.stats.Discarded += discarded
}

// Run reads from src until stop is closed, reopening with back-off (1 s doubling to
// 30 s) whenever the source closes or fails (FRAME_FORMAT.md section 4).
func Run(src Source, in *Ingest, stop <-chan struct{}) { RunLink(src, in, nil, stop) }

// RunLink is Run that also attaches link to the open source, so downlink frames can be
// written (FRAME_FORMAT.md section 7). link may be nil.
func RunLink(src Source, in *Ingest, link *Link, stop <-chan struct{}) {
	backoff := time.Second
	for {
		select {
		case <-stop:
			return
		default:
		}
		rc, atBoundary, err := src.Open()
		if err != nil {
			log.Printf("open %s: %v (retry in %v)", src, err, backoff)
		} else {
			log.Printf("reading frames from %s", src)
			backoff = time.Second
			if link != nil {
				link.attach(src, rc)
			}
			lr := frame.NewLineReader(rc, atBoundary)
			for {
				line, err := lr.Next()
				if err != nil {
					if !errors.Is(err, io.EOF) {
						log.Printf("read %s: %v", src, err)
					}
					break
				}
				in.Line(line)
			}
			if link != nil {
				link.detach()
			}
			rc.Close()
			in.SourceClosed(lr.Discarded)
			log.Printf("source %s closed (retry in %v)", src, backoff)
		}
		select {
		case <-stop:
			return
		case <-time.After(backoff):
		}
		if backoff *= 2; backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}
