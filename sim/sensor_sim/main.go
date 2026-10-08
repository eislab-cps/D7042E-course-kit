// Command sensor_sim emits FRAME_FORMAT.md frames for one assignment scenario, as the
// Pico would over UART, to stdout, a named pipe or a TCP socket.
//
//	go run ./sim/sensor_sim -scenario cold-chain -out tcp://localhost:7000
//	go run ./sim/sensor_sim -scenario press -out pipe:/tmp/pico -period 500ms
//	FRAME_HMAC_KEY=<64 hex> go run ./sim/sensor_sim -hmac ...
//	go run ./sim/sensor_sim -actuator cooling -out tcp://localhost:7000   # R8: downlink on the same connection
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
)

func main() {
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stderr)
	var (
		scenario  = flag.String("scenario", "cold-chain", "one of: "+strings.Join(ScenarioNames(), ", "))
		out       = flag.String("out", "stdout", "stdout, pipe:<path> or tcp://<host:port>")
		period    = flag.Duration("period", time.Second, "sample period")
		count     = flag.Int("count", 0, "frames to emit, 0 = forever")
		threshold = flag.Float64("threshold", 0, "alert threshold for the primary quantity in physical units; events cross it (0 = scenario default)")
		every     = flag.Int("excursion-every", 60, "samples between threshold-crossing events, 0 = none")
		length    = flag.Int("excursion-len", 5, "samples each event lasts")
		seed      = flag.Int64("seed", time.Now().UnixNano(), "random seed")
		useHMAC   = flag.Bool("hmac", false, "append HMAC tags (key from FRAME_HMAC_KEY)")
		tamper    = flag.Int("tamper-every", 0, "change a value after tagging in every Nth frame, to test HMAC checking (0 = never)")
		actuator  = flag.String("actuator", "", "R8: simulate an actuator, cooling or valve (FRAME_FORMAT.md section 7); empty = none")
		actDelay  = flag.Duration("act-delay", 500*time.Millisecond, "R8: time before the observed state (ACTS) follows a command")
		actFault  = flag.String("act-fault", "", "R8: stuck = acknowledge commands but never move")
		linkTO    = flag.Duration("link-timeout", 10*time.Second, "R8: enter the safe state after this long without a valid downlink frame")
		in        = flag.String("in", "", "R8 downlink input: stdin, pipe:<path> or none (default: the TCP connection; <pipe>.down with -out pipe:; stdin with -out stdout)")
	)
	flag.Parse()

	sc, err := Lookup(*scenario)
	if err != nil {
		log.Fatal(err)
	}
	if *threshold != 0 {
		sc = sc.WithThreshold(*threshold)
	}
	var key []byte
	if *useHMAC || (*actuator != "" && os.Getenv("FRAME_HMAC_KEY") != "") {
		if key, err = frame.ParseKey(os.Getenv("FRAME_HMAC_KEY")); err != nil {
			log.Fatal(err)
		}
	}
	uplinkKey := key
	if !*useHMAC {
		uplinkKey = nil // the key only verifies tagged downlink frames
	}
	sink, err := OpenSink(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer sink.Close()

	g := NewGenerator(sc, Config{ExcursionEvery: *every, ExcursionLen: *length}, *seed)

	var act *Actuator
	if *actuator != "" {
		if *actFault != "" && *actFault != "stuck" {
			log.Fatalf("-act-fault must be stuck or empty, got %q", *actFault)
		}
		act, err = NewActuator(*actuator, *actDelay, *linkTO, *actFault == "stuck", key, *useHMAC, time.Now)
		if err != nil {
			log.Fatal(err)
		}
		if err := startDownlink(*in, *out, sink, act.Downlink); err != nil {
			log.Fatal(err)
		}
		// Cold chain: with cooling off the room warms by 0.08 °C per sample, up to +15 °C;
		// with cooling on it returns towards its setpoint.
		var warm float64
		g.Offset = func(k string) float64 {
			if k != "TEMP" || act.Kind != "cooling" {
				return 0
			}
			if act.Observed() == 0 {
				warm = math.Min(warm+0.08, 15)
			} else {
				warm *= 0.9
			}
			return warm
		}
		log.Printf("actuator %s, safe value %d, delay %v, link timeout %v, fault %q",
			act.Kind, act.SafeValue, *actDelay, *linkTO, *actFault)
	}
	log.Printf("scenario %s, primary %s, threshold %v, period %v, hmac %v",
		sc.Name, sc.Primary, sc.Threshold, *period, uplinkKey != nil)
	if *tamper > 0 && uplinkKey == nil {
		log.Printf("warning: -tamper-every without -hmac: tampered frames are undetectable")
	}
	tick := time.NewTicker(*period)
	defer tick.Stop()
	for n := 0; *count == 0 || n < *count; n++ {
		f := g.Next()
		if act != nil {
			act.Tick()
			f.Fields = append(f.Fields, act.Fields()...)
		}
		line := frame.Encode(f, uplinkKey)
		if *tamper > 0 && (n+1)%*tamper == 0 {
			line = Tamper(line)
			log.Printf("tampered frame: %s", strings.TrimSpace(line))
		}
		if err := sink.Write(line); err != nil {
			log.Fatal(fmt.Errorf("write: %w", err))
		}
		if *count == 0 || n+1 < *count {
			<-tick.C
		}
	}
}

// startDownlink connects the actuator to its downlink input (FRAME_FORMAT.md section 7).
func startDownlink(in, out string, sink Sink, h func(string)) error {
	if in == "none" {
		return nil
	}
	if in == "" {
		switch {
		case strings.HasPrefix(out, "tcp://"):
			ds, ok := sink.(DownlinkSink)
			if !ok {
				return fmt.Errorf("tcp sink cannot read the downlink")
			}
			ds.Downlink(h)
			return nil
		case strings.HasPrefix(out, "pipe:"):
			in = out + ".down"
		default:
			in = "stdin"
		}
	}
	return OpenDownlink(in, h)
}
