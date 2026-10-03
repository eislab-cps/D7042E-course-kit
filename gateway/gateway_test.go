package main

import (
	"errors"
	"testing"

	"github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu/transport"
)

// The Arrowhead steps are explicit TODOs: start-up stops at step 1 with a clear message,
// and the other steps report errTODO too.
func TestArrowheadStepsAreTODO(t *testing.T) {
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
