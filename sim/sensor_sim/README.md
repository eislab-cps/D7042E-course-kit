# sim/sensor_sim — device simulator

Emits the same UART frames as the Pico firmware (`FRAME_FORMAT.md`), for one assignment
scenario, to stdout, a named pipe or a TCP socket. The gateway reads them with the same
parser (`frame/`) it uses for real hardware and cannot tell the difference.

```bash
go run ./sim/sensor_sim -scenario cold-chain -out tcp://localhost:7000   # gateway: SERIAL_SOURCE=tcp://localhost:7000
go run ./sim/sensor_sim -scenario press -out pipe:/tmp/pico              # gateway: SERIAL_SOURCE=pipe (path /tmp/pico)
go run ./sim/sensor_sim -scenario energy -count 5 -period 100ms          # print five frames
```

| Flag | Default | Meaning |
|---|---|---|
| `-scenario` | `cold-chain` | `cold-chain`, `energy`, `air-quality`, `press` |
| `-out` | `stdout` | `stdout`, `pipe:<path>` (created with `mkfifo` if missing; Linux/macOS/WSL), `tcp://<host:port>` (the simulator listens; every connected client gets every frame from the next one on) |
| `-period` | `1s` | sample period |
| `-count` | `0` | frames to emit, `0` = forever |
| `-threshold` | scenario default | alert threshold for the primary quantity, physical units |
| `-excursion-every` | `60` | samples between threshold-crossing events, `0` = none |
| `-excursion-len` | `5` | samples each event lasts |
| `-seed` | time | random seed, for reproducible runs |
| `-hmac` | off | append HMAC tags (`FRAME_FORMAT.md` section 5); key from `FRAME_HMAC_KEY` (64 hex digits) |

Scenarios (keys and scaling in `FRAME_FORMAT.md` section 3):

| Scenario | Keys | Primary | Default threshold | Steady state |
|---|---|---|---|---|
| `cold-chain` | `TEMP`, `HUM` | `TEMP` | 8.0 °C | about 4 °C, 60 %RH |
| `energy` | `PWR`, `OCC` | `PWR` | 4500 W | about 3000 W; occupancy toggles |
| `air-quality` | `CO2`, `TEMP` | `CO2` | 1000 ppm | about 650 ppm, 22 °C |
| `press` | `PRES`, `CYC` | `PRES` | 230 bar | about 200 bar; cycle count +1 per sample |

The primary quantity stays below its threshold in steady state and crosses it during
each event, so an alert path has something to react to. The first frame carries `BOOT`;
sequence numbers wrap at 65535.

Tests: `go test ./sim/... ./frame/...` checks every emitted frame of every scenario,
with and without HMAC, against the shared parser.
