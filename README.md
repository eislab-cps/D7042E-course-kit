# D7042E course kit

Starter kit for the LTU course **D7042E** (second cycle): an industrial IoT system
built on the Arrowhead 5.2 core systems from
[Arrowhead-520-Go-Evol](https://github.com/ulfbod/Arrowhead-520-Go-Evol), using the
[educational Go SDK](https://github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu).

Everything runs on a laptop. Real Raspberry Pi hardware is optional and a drop-in
replacement: the same gateway code reads from a simulator or from a serial port.

**Status:** ready for the course. The kit pins the Arrowhead stack at **v0.1.1**
(prebuilt images `ghcr.io/ulfbod/<name>:v0.1.1`) and the SDK at **v0.1.0** (`go.mod`).
It contains the compose stack, a working sensor simulator, a gateway stub that already
reads and checks device frames, and a Wokwi skeleton. What you build is described in
`assignment/ASSIGNMENT.md`.

## Layout

```
deploy/           slim docker compose: 3 foundation systems, orchestrator (consumerauth
                  mode), profile-ca, cert-provisioner, Mosquitto, InfluxDB (pinned images)
sim/sensor_sim/   Go program emitting Pico UART frames to a named pipe or TCP socket
gateway/          Go stub for the RPi 4 role; SERIAL_SOURCE selects pipe, socket, /dev/ttyUSB0
wokwi/pico_sensor Wokwi project: Pico + BMP180 + DHT22 + LED; C skeleton (I2C and UART init only)
assignment/       ASSIGNMENT.md with requirements R1–R12; api-quick-reference.md
FRAME_FORMAT.md   the UART frame contract shared by simulator, gateway and Wokwi code
```

## Quick start

You need Docker with Compose v2 and Go 1.25 or newer. Run from the repository root.

Terminal 1 — start the Arrowhead local cloud, Mosquitto and InfluxDB:

```bash
cp deploy/.env.example deploy/.env        # lab-only values
docker compose -f deploy/docker-compose.yml up -d --wait
curl -s http://localhost:8787/health      # {"status":"ok","system":"profile-ca"}
go run ./sim/sensor_sim -scenario cold-chain -out tcp://localhost:7000
```

Terminal 2 — run the gateway stub against the simulator:

```bash
SERIAL_SOURCE=tcp://localhost:7000 go run ./gateway
```

The gateway logs `reading frames from tcp://localhost:7000` and
`Arrowhead start-up stopped: step 1 certificate: TODO, see gateway/arrowhead.go` (in either
order), and every 30 s a line of frame counters such as `counters {Accepted:30 Lost:0 …}`. That TODO is where the assignment starts: read
`assignment/ASSIGNMENT.md` and `assignment/api-quick-reference.md`.

Stop with Ctrl-C in both terminals, then remove the stack and all its state:

```bash
docker compose -f deploy/docker-compose.yml down -v
```

## Development

```bash
go vet ./...      # tier 1
go test ./...     # tier 2
```

## Documentation

| File | What it covers |
|---|---|
| `assignment/ASSIGNMENT.md` | The assignment: brief, scenarios, requirements R1–R12, phases, submission, oral examination |
| `assignment/api-quick-reference.md` | Endpoints, request shapes and `curl` examples for the kit's Arrowhead systems |
| `FRAME_FORMAT.md` | Device frame contract between Pico/simulator and gateway |
| `deploy/README.md` | The compose stack: services, ports, settings, `.env` |
| `sim/sensor_sim/README.md` | Simulator flags and scenarios |
| `gateway/README.md` | Gateway stub: `SERIAL_SOURCE`, environment, what is already done |
| `wokwi/pico_sensor/README.md` | Wokwi project: wiring, running in the browser, building locally |
| `ARCHITECTURE.md` | Kit structure and the course pipeline |

## For students

Create your own repository from this one with GitHub's **Use this template** button
(not a fork). Keep it private and share it with the examiner.

## License

MIT, see `LICENSE`.
