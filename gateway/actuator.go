package main

// The actuator side of the gateway (assignment R8, for grades 4 and 5). Nothing here runs
// until you wire it into main. The device side is done for you: device.Link writes one
// line to the device over the same serial device or TCP connection the frames arrive on
// (or <pipe>.down in pipe mode), and the frame package encodes downlink frames
// (FRAME_FORMAT.md section 7). The shapes are in api-quick-reference.md, "Actuation (R8)".
//
// Keep three questions apart, in three places (ASSIGNMENT.md, R8 levels):
//   authorization  may this consumer use coolingControl?  ConsumerAuthorization verify
//   arbitration    which permitted command runs now?      your Controller (grade 5)
//   execution      did the device actually move?          the device's ACTS and ACK

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/eislab-cps/D7042E-course-kit/frame"
	"github.com/eislab-cps/D7042E-course-kit/gateway/device"
)

// Downlink frames commands for the device: its own C sequence (starting at 0), BOOT on
// the first frame after a gateway start, and the HMAC tag when a key is set.
type Downlink struct {
	Link *device.Link
	Key  []byte // the FRAME_HMAC_KEY; a device that tags its uplink requires tagged downlink
	Boot int32  // 1..65535, new on every gateway start (e.g. from the clock)

	mu     sync.Mutex
	seq    uint16
	booted bool
}

// Command sends C:<seq>;CID:<cid>;ACT:<act>.
//
// TODO (R8 minimal): build a frame.Command{Kind: frame.KindAct, ...}, take its Fields
// (with d.Boot on the first frame only), encode with frame.EncodeDownlink(f, d.Key),
// write it with d.Link.WriteLine, and advance the sequence. Return device.ErrNoLink
// unchanged so the handler can answer 503.
func (d *Downlink) Command(cid, act int32) error {
	_ = frame.EncodeDownlink
	return errTODO
}

// Safe sends C:<seq>;CID:<cid>;SAFE:1: the device enters its safe state at once.
//
// TODO (R8 coordinated): as Command, with frame.KindSafe.
func (d *Downlink) Safe(cid int32) error { return errTODO }

// KeepAlive sends C:<seq>;KA:1. Without a valid downlink frame for LINK_TIMEOUT (10 s)
// the device enters its safe state, so send one at least every LINK_TIMEOUT/3.
//
// TODO (R8 minimal): as Command, with frame.KindKeepAlive; then call it from a
// goroutine in main on a time.Ticker, ignoring device.ErrNoLink.
func (d *Downlink) KeepAlive() error { return errTODO }

// CommandState is what the actuator service reports: requested is what the consumer
// asked for, commanded is what the gateway sent to the device, observed is what the
// device reports (ACTS) after it acknowledged the command (ACK). They differ while the
// device is moving, when a command was refused, and with sim -act-fault stuck.
type CommandState struct {
	CID       int32 `json:"cid"`
	Requested int32 `json:"requested"`
	Commanded int32 `json:"commanded"`
	Observed  int32 `json:"observed"`
	Ack       int32 `json:"ack"`
	Safe      bool  `json:"safe"`
}

// observed reads the device's side of CommandState from the latest frames.
func observed(in *device.Ingest) (acts, ack int32, safe, ok bool) {
	a, okA := in.Latest("ACTS")
	k, okK := in.Latest("ACK")
	s, _ := in.Latest("SAFE")
	return a.Raw, k.Raw, s.Raw == 1, okA && okK
}

// serveActuator runs the actuator service (coolingControl) on its own HTTPS port.
//
// TODO (R8 minimal), in this order (the refusal table in api-quick-reference.md):
//  1. an http.Server with tls.Config{ClientAuth: tls.RequireAndVerifyClientCert,
//     ClientCAs: the profile-ca root, Certificates: the gateway's own certificate};
//  2. POST /cooling/command: the caller is r.TLS.PeerCertificates[0].Subject.CommonName;
//     ask ConsumerAuthorization verify (403 on false); decode {"cid","act"} (400);
//     a cid already handled answers 200 with its first result and sends nothing;
//     dl.Command (503 on device.ErrNoLink); answer 202 with the CommandState;
//  3. GET /cooling/state: the CommandState, observed from the latest ACTS, ACK and SAFE.
//
// Register coolingControl (CERT_AUTH) and its rule in arrowhead.go, as in step 3.
//
// TODO (R8 coordinated): route every command through a Controller (below) before
// dl.Command, and add the lease and e-stop endpoints.
func serveActuator(ctx context.Context, addr string, in *device.Ingest, dl *Downlink) error {
	_ = http.StatusAccepted
	_ = observed
	return errTODO
}

// Controller arbitrates between consumers that are all permitted to use the actuator
// (R8 coordinated, grade 5). ConsumerAuthorization cannot do this: it answers "may this
// consumer use the service?", not "whose command runs now?". The implementation is
// yours; the kit gives the interface only.
//
// The properties to show, with the refusals of api-quick-reference.md:
//   - one lease holder at a time; every grant gets a higher epoch (fencing token);
//   - a command with a stale epoch is refused, even after a partition heals;
//   - the e-stop preempts any lease and latches until released;
//   - a repeated cid is executed once;
//   - on lease expiry, e-stop or link loss the device goes to its safe state.
type Controller interface {
	Acquire(consumer string, ttl time.Duration) (epoch uint64, err error)
	Renew(consumer string, epoch uint64) error
	Release(consumer string, epoch uint64) error
	// Admit decides whether a permitted command runs now; it does not send it.
	Admit(consumer string, epoch uint64, cid int32) error
	EStop(cid int32) error
	ReleaseEStop() error
}
