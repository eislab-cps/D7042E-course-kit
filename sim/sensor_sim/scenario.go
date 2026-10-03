package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

// Channel models one quantity in physical units; the generator scales it to the
// FRAME_FORMAT.md integer for its key.
type Channel struct {
	Key       string
	Base      float64 // steady-state physical value
	Noise     float64 // standard deviation of per-sample noise
	Drift     float64 // slow random-walk step per sample
	Excursion float64 // added during a threshold-crossing event (0: channel has none)
	Counter   bool    // monotonic counter (CYC): +1 per sample, no noise
	Binary    bool    // 0/1 state (OCC): flips with probability Noise per sample
}

// Scenario is one row of the assignment's scenario table.
type Scenario struct {
	Name      string
	Channels  []Channel
	Primary   string  // key the threshold applies to
	Threshold float64 // physical value that alerts are defined against
}

// Scenarios are the four assignment scenarios, keyed by -scenario name.
var Scenarios = map[string]Scenario{
	"cold-chain": {
		Name: "cold-chain", Primary: "TEMP", Threshold: 8.0,
		Channels: []Channel{
			{Key: "TEMP", Base: 4.0, Noise: 0.15, Drift: 0.01, Excursion: 6.0},
			{Key: "HUM", Base: 60.0, Noise: 1.0, Drift: 0.05},
		},
	},
	"energy": {
		Name: "energy", Primary: "PWR", Threshold: 4500,
		Channels: []Channel{
			{Key: "PWR", Base: 3000, Noise: 150, Drift: 5, Excursion: 2500},
			{Key: "OCC", Binary: true, Noise: 0.02},
		},
	},
	"air-quality": {
		Name: "air-quality", Primary: "CO2", Threshold: 1000,
		Channels: []Channel{
			{Key: "CO2", Base: 650, Noise: 20, Drift: 2, Excursion: 700},
			{Key: "TEMP", Base: 22.0, Noise: 0.1, Drift: 0.01},
		},
	},
	"press": {
		Name: "press", Primary: "PRES", Threshold: 230,
		Channels: []Channel{
			{Key: "PRES", Base: 200, Noise: 3, Drift: 0.2, Excursion: 45},
			{Key: "CYC", Counter: true},
		},
	},
}

// WithThreshold returns a copy whose primary threshold is t (physical units). The
// primary channel's event size moves with it, keeping the scenario's default overshoot,
// so events still cross the new threshold and steady state stays below it.
func (sc Scenario) WithThreshold(t float64) Scenario {
	chs := append([]Channel(nil), sc.Channels...)
	for i, ch := range chs {
		if ch.Key == sc.Primary && ch.Excursion != 0 {
			overshoot := ch.Base + ch.Excursion - sc.Threshold
			chs[i].Excursion = t - ch.Base + overshoot
		}
	}
	sc.Channels, sc.Threshold = chs, t
	return sc
}

// ScenarioNames returns the scenario names in stable order.
func ScenarioNames() []string {
	var n []string
	for k := range Scenarios {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

// Config controls event timing. ExcursionEvery 0 disables threshold-crossing events.
type Config struct {
	ExcursionEvery int // samples between event starts
	ExcursionLen   int // samples an event lasts
}

// Generator produces the frame sequence for one scenario.
type Generator struct {
	sc     Scenario
	cfg    Config
	rng    *rand.Rand
	seq    uint16
	sample int
	drift  []float64
	state  []float64 // binary state or counter value
	boot   int32
}

// NewGenerator seeds a generator. The first frame carries BOOT with a random epoch.
func NewGenerator(sc Scenario, cfg Config, seed int64) *Generator {
	g := &Generator{sc: sc, cfg: cfg, rng: rand.New(rand.NewSource(seed))}
	g.drift = make([]float64, len(sc.Channels))
	g.state = make([]float64, len(sc.Channels))
	g.boot = int32(1 + g.rng.Intn(65535))
	return g
}

// InExcursion reports whether sample n falls inside a threshold-crossing event.
func (g *Generator) InExcursion(n int) bool {
	if g.cfg.ExcursionEvery <= 0 || g.cfg.ExcursionLen <= 0 {
		return false
	}
	return n >= g.cfg.ExcursionEvery && n%g.cfg.ExcursionEvery < g.cfg.ExcursionLen
}

// Next returns the next frame.
func (g *Generator) Next() frame.Frame {
	f := frame.Frame{Seq: g.seq}
	if g.sample == 0 {
		f.Fields = append(f.Fields, frame.Field{Key: "BOOT", Value: g.boot})
	}
	exc := g.InExcursion(g.sample)
	for i, ch := range g.sc.Channels {
		spec := frame.Keys[ch.Key]
		var raw int64
		switch {
		case ch.Counter:
			raw = int64(g.state[i])
			g.state[i]++
			if g.state[i] > float64(spec.Max) {
				g.state[i] = 0
			}
		case ch.Binary:
			if g.rng.Float64() < ch.Noise {
				g.state[i] = 1 - g.state[i]
			}
			raw = int64(g.state[i])
		default:
			g.drift[i] += g.rng.NormFloat64() * ch.Drift
			// Keep the random walk near the base so the next event is a clear crossing.
			g.drift[i] *= 0.98
			v := ch.Base + g.drift[i] + g.rng.NormFloat64()*ch.Noise
			if exc {
				v += ch.Excursion
			}
			raw = int64(math.Round(v * float64(spec.Scale)))
		}
		if raw < int64(spec.Min) {
			raw = int64(spec.Min)
		}
		if raw > int64(spec.Max) {
			raw = int64(spec.Max)
		}
		f.Fields = append(f.Fields, frame.Field{Key: ch.Key, Value: int32(raw)})
	}
	g.seq++ // wraps 65535 -> 0
	g.sample++
	return f
}

// Lookup returns the scenario for name.
func Lookup(name string) (Scenario, error) {
	sc, ok := Scenarios[name]
	if !ok {
		return Scenario{}, fmt.Errorf("unknown scenario %q (have %v)", name, ScenarioNames())
	}
	return sc, nil
}

// Tamper changes the first data value of an encoded frame line by +1 and leaves the rest,
// including any HMAC tag, untouched. Applied after tagging, it produces a frame whose tag
// no longer matches: what an attacker on the line would send (R10 option B demo).
func Tamper(line string) string {
	body := strings.TrimSuffix(line, "\n")
	parts := strings.Split(body, ";")
	for i, p := range parts {
		k, v, ok := strings.Cut(p, ":")
		if !ok || k == "S" || k == "BOOT" || k == "H" {
			continue
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		parts[i] = k + ":" + strconv.FormatInt(n+1, 10)
		break
	}
	return strings.Join(parts, ";") + "\n"
}
