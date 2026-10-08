package main

import (
	"errors"
	"testing"

	"github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu/transport"
	"github.com/eislab-cps/D7042E-course-kit/frame"
	"github.com/eislab-cps/D7042E-course-kit/gateway/device"
)

// REPLACE THIS TEST when you implement the Arrowhead steps (assignment Phase 1).
//
// It checks the stub as handed out: start-up stops at step 1 with a clear message and the
// other steps report errTODO. Once obtainCertificate, login and registerServices work it
// fails, by design. Delete it then and test your own implementation instead, or CI on your
// repository turns red.
func TestStubStepsStillTODO_ReplaceMe(t *testing.T) {
	err := arrowheadStartup(t.Context(), ArrowheadConfig{SystemName: "ColdChainGateway"})
	if err == nil || err.Error() != "step 1 certificate: TODO, see gateway/arrowhead.go" {
		t.Errorf("err %v", err)
	}
	if _, err := login(t.Context(), ArrowheadConfig{}, transport.Credentials{}); !errors.Is(err, errTODO) {
		t.Errorf("login: %v", err)
	}
	if err := registerServices(t.Context(), ArrowheadConfig{}, transport.Credentials{}, ""); !errors.Is(err, errTODO) {
		t.Errorf("register: %v", err)
	}
}

// REPLACE THIS TEST when you implement R8 (grades 4 and 5); it is fine as it is for grade 3.
// It checks that the actuator side is still TODO as handed out.
func TestActuatorStillTODO_ReplaceMe(t *testing.T) {
	dl := &Downlink{Link: &device.Link{}}
	if err := dl.Command(1, 1); !errors.Is(err, errTODO) {
		t.Errorf("Command: %v", err)
	}
	if err := dl.KeepAlive(); !errors.Is(err, errTODO) {
		t.Errorf("KeepAlive: %v", err)
	}
	if err := serveActuator(t.Context(), ":0", device.NewIngest(frame.HMACOff, nil), dl); !errors.Is(err, errTODO) {
		t.Errorf("serveActuator: %v", err)
	}
}
