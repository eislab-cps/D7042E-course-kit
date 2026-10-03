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
frame checks, counters, latest readings); `main.go` wires it up.

What the stub already does:

- opens the source and reopens it with back-off (1 s doubling to 30 s) when it closes;
- discards a partial first line on a serial device;
- drops malformed, duplicate or stale frames and frames failing the HMAC check;
- drops out-of-range fields; counts lost frames from sequence gaps;
- logs every drop and prints the counters every 30 s.

Tests: `go test ./gateway/...` covers source selection, pipe/TCP/serial opening, every
counter, reconnecting, and that the three Arrowhead steps are still TODO.
