# gateway/ — gateway stub (the RPi 4 role)

Reads device frames, applies the `FRAME_FORMAT.md` rules with the shared `frame/`
package, and keeps the latest reading per key. The parts that make it an Arrowhead
system are yours to write: three explicit steps in `arrowhead.go` (certificate,
identity, registration) and the HTTP endpoints marked `TODO` in `main.go`. The SDK is
`github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu`, pinned in the root `go.mod`.

```bash
go run ./sim/sensor_sim -scenario cold-chain -out tcp://localhost:7000 &
SERIAL_SOURCE=tcp://localhost:7000 go run ./gateway
```

| Variable | Default | Meaning |
|---|---|---|
| `SERIAL_SOURCE` | `pipe` | `pipe` (named pipe `FRAME_PIPE`, default `/tmp/pico`), `pipe:<path>`, `tcp://<host:port>`, or a serial device such as `/dev/ttyUSB0` (set the line first: `stty -F /dev/ttyUSB0 115200 raw`) |
| `FRAME_HMAC` | `off` | `off`, `optional` or `required` (`FRAME_FORMAT.md` section 5) |
| `FRAME_HMAC_KEY` | — | 64 hex digits, needed unless `FRAME_HMAC=off` |
| `SYSTEM_NAME` | `ColdChainGateway` | PascalCase system name, also the certificate name |
| `SYSTEM_PASSWORD` | — | the password Sysop gave this system's identity |
| `CA_PLAIN_URL`, `CA_TLS_URL` | `http://localhost:8787`, `https://localhost:8788` | profile-ca |
| `SR_URL`, `AUTH_URL` | `https://localhost:8490`, `https://localhost:8491` | ServiceRegistry and Authentication; the SDK's `transport.NewMTLS` takes the TLS server name (`serviceregistry`, `authentication`) |
| `CERT_DIR` | `certs` | where the certificate, key and `ca.crt` are kept |
| `ADVERTISE_ADDRESS`, `GATEWAY_PORT` | `localhost`, `9443` | what you register as `accessAddresses` / `accessPort` |

The device side lives in the importable package `gateway/device` (source selection,
frame checks, counters, latest readings, and `Link`, the write side of the device
connection); `main.go` wires it up.

## Actuation (R8, grades 4 and 5)

`actuator.go` holds the actuator side as TODOs: the `Downlink` writer (sequence, `BOOT`,
tag, keep-alive), the actuator service `serveActuator` (HTTPS with client certificates,
ConsumerAuthorization verify, requested, commanded and observed state), and, for grade 5,
the `Controller` interface for leases, fencing epochs and the e-stop. Shapes:
`../assignment/api-quick-reference.md`, "Actuation (R8)"; frames: `../FRAME_FORMAT.md`
section 7.

`device.Link` already writes downlink lines to the device, over the same connection as the
uplink:

| `SERIAL_SOURCE` | Downlink goes to |
|---|---|
| `tcp://host:port` | the same TCP connection |
| `/dev/ttyUSB0` (any serial device) | the same device, opened read-write |
| `pipe` or `pipe:<path>` | a second named pipe `<path>.down` (the simulator reads it) |

`WriteLine` returns `device.ErrNoLink` while no device is connected (and, in pipe mode,
while nothing reads `<path>.down`); answer such a command with `503`.

```bash
go run ./sim/sensor_sim -scenario cold-chain -actuator cooling -out tcp://localhost:7000 &
SERIAL_SOURCE=tcp://localhost:7000 go run ./gateway
```

Until your gateway sends keep-alives, the simulator stays in its safe state (`SAFE:1`).

What the stub already does:

- opens the source and reopens it with back-off (1 s doubling to 30 s) when it closes;
- discards a partial first line on a serial device;
- drops malformed, duplicate or stale frames and frames failing the HMAC check;
- drops out-of-range fields; counts lost frames from sequence gaps;
- logs every drop and prints the counters every 30 s;
- keeps the device connection writable for downlink frames (`device.Link`).

Tests: `go test ./gateway/...` covers source selection, pipe/TCP/serial opening, every
counter, reconnecting, downlink writes over TCP, a named pipe and a serial device (a
pseudo-terminal, Linux), and that the Arrowhead steps and the actuator side are still TODO.
`TestStubStepsStillTODO_ReplaceMe` in `gateway_test.go` checks the stub as handed out, so it
fails once you implement the steps: replace it with tests for your implementation when you
do, or CI on your repository turns red. `TestActuatorStillTODO_ReplaceMe` does the same for
R8; leave it if you aim for grade 3.
