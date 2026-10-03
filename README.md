# D7042E course kit

Starter kit for the LTU course **D7042E** (second cycle): an industrial IoT system
built on the Arrowhead 5.2 core systems from
[Arrowhead-520-Go-Evol](https://github.com/ulfbod/Arrowhead-520-Go-Evol), using the
[educational Go SDK](https://github.com/eislab-cps/Arrowhead-520-Evol-Go-SDK-Edu).

Everything runs on a laptop. Real Raspberry Pi hardware is optional and a drop-in
replacement: the same gateway code reads from a simulator or from a serial port.

**Status:** skeleton. No code yet.

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

```bash
# not yet available
```

## Development

```bash
go vet ./...      # tier 1
go test ./...     # tier 2
```

## Documentation

| File | What it covers |
|---|---|
| `ARCHITECTURE.md` | Kit structure and the course pipeline |
| `FRAME_FORMAT.md` | Sensor frame contract |

## For students

Create your own repository from this one with GitHub's **Use this template** button
(not a fork). Keep it private and share it with the examiner.

## License

MIT, see `LICENSE`.
