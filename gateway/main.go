// Command gateway is the D7042E gateway stub (the RPi 4 role). It reads device frames
// from SERIAL_SOURCE through the shared FRAME_FORMAT implementation and keeps the latest
// reading per key. The Arrowhead steps (certificate, identity, registration) and the
// HTTP service endpoints are yours to implement; see arrowhead.go and the TODOs below.
// For R8 (grades 4 and 5), actuator.go holds the actuator side.
//
//	SERIAL_SOURCE=tcp://localhost:7000 go run ./gateway
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
	"github.com/eislab-cps/D7042E-course-kit/gateway/device"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(log.Ltime)

	src, err := device.ParseSource(os.Getenv("SERIAL_SOURCE"), os.Getenv("FRAME_PIPE"))
	if err != nil {
		log.Fatal(err)
	}
	mode, err := frame.ParseHMACMode(os.Getenv("FRAME_HMAC"))
	if err != nil {
		log.Fatal(err)
	}
	var key []byte
	if mode != frame.HMACOff {
		if key, err = frame.ParseKey(os.Getenv("FRAME_HMAC_KEY")); err != nil {
			log.Fatal(err)
		}
	}
	port, _ := strconv.Atoi(env("GATEWAY_PORT", "9443"))
	cfg := ArrowheadConfig{
		SystemName: env("SYSTEM_NAME", "ColdChainGateway"),
		Password:   os.Getenv("SYSTEM_PASSWORD"),
		CAPlainURL: env("CA_PLAIN_URL", "http://localhost:8787"),
		CATLSURL:   env("CA_TLS_URL", "https://localhost:8788"),
		SRURL:      env("SR_URL", "https://localhost:8490"),
		AuthURL:    env("AUTH_URL", "https://localhost:8491"),
		CertDir:    env("CERT_DIR", "certs"),
		AdvertAddr: env("ADVERTISE_ADDRESS", "localhost"),
		AdvertPort: port,
	}

	in := device.NewIngest(mode, key)
	link := &device.Link{} // the write side of the same device connection (R8)
	stop := make(chan struct{})
	go device.RunLink(src, in, link, stop)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := arrowheadStartup(ctx, cfg); err != nil {
		log.Printf("Arrowhead start-up stopped: %v", err)
	}
	cancel()

	// TODO: start an HTTP(S) server on cfg.AdvertPort with one endpoint per sensor
	// quantity, e.g. GET /temperature returning in.Latest("TEMP") as JSON with the
	// field names and units of your interface contract.

	// TODO (R8): dl := &Downlink{Link: link, Key: key, Boot: <1..65535, new per start>};
	// start the keep-alive goroutine (dl.KeepAlive at least every 10s/3) and
	// serveActuator(ctx, ":9444", in, dl). See actuator.go.

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-sig:
			close(stop)
			log.Printf("stopped; counters %+v", in.Stats())
			return
		case <-tick.C:
			log.Printf("counters %+v", in.Stats())
		}
	}
}

// arrowheadStartup runs the three explicit steps in order.
func arrowheadStartup(ctx context.Context, cfg ArrowheadConfig) error {
	creds, err := obtainCertificate(ctx, cfg)
	if err != nil {
		return stepErr("step 1 certificate", err)
	}
	token, err := login(ctx, cfg, creds)
	if err != nil {
		return stepErr("step 2 identity", err)
	}
	if err := registerServices(ctx, cfg, creds, token); err != nil {
		return stepErr("step 3 registration", err)
	}
	log.Printf("registered as %s", cfg.SystemName)
	return nil
}

func stepErr(step string, err error) error {
	if errors.Is(err, errTODO) {
		return errors.New(step + ": TODO, see gateway/arrowhead.go")
	}
	return errors.New(step + ": " + err.Error())
}
