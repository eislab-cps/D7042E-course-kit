// Command sensor_sim emits FRAME_FORMAT.md frames for one assignment scenario, as the
// Pico would over UART, to stdout, a named pipe or a TCP socket.
//
//	go run ./sim/sensor_sim -scenario cold-chain -out tcp://localhost:7000
//	go run ./sim/sensor_sim -scenario press -out pipe:/tmp/pico -period 500ms
//	FRAME_HMAC_KEY=<64 hex> go run ./sim/sensor_sim -hmac ...
package main

import (
	"flag"
	"fmt"
	"log"
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
	if *useHMAC {
		if key, err = frame.ParseKey(os.Getenv("FRAME_HMAC_KEY")); err != nil {
			log.Fatal(err)
		}
	}
	sink, err := OpenSink(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer sink.Close()

	g := NewGenerator(sc, Config{ExcursionEvery: *every, ExcursionLen: *length}, *seed)
	log.Printf("scenario %s, primary %s, threshold %v, period %v, hmac %v",
		sc.Name, sc.Primary, sc.Threshold, *period, key != nil)
	if *tamper > 0 && key == nil {
		log.Printf("warning: -tamper-every without -hmac: tampered frames are undetectable")
	}
	tick := time.NewTicker(*period)
	defer tick.Stop()
	for n := 0; *count == 0 || n < *count; n++ {
		line := frame.Encode(g.Next(), key)
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
