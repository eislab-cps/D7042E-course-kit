# sim/sensor_sim — device simulator

Emits the same UART frames as the Pico firmware (`FRAME_FORMAT.md`), for one assignment
scenario, to stdout, a named pipe or a TCP socket. The gateway reads them with the same
parser (`frame/`) it uses for real hardware and cannot tell the difference.

```bash
go run ./sim/sensor_sim -scenario cold-chain -out tcp://localhost:7000   # gateway: SERIAL_SOURCE=tcp://localhost:7000
go run ./sim/sensor_sim -scenario press -out pipe:/tmp/pico              # gateway: SERIAL_SOURCE=pipe (path /tmp/pico)
go run ./sim/sensor_sim -scenario energy -count 5 -period 100ms          # print five frames
go run ./sim/sensor_sim -actuator cooling -out tcp://localhost:7000      # R8: with an actuator
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
| `-tamper-every` | `0` | every Nth frame, change one value after tagging, so the tag no longer matches (test your gateway's `FRAME_HMAC=required`; use with `-hmac`) |
| `-actuator` | none | R8: simulate an actuator, `cooling` or `valve` (`FRAME_FORMAT.md` section 7) |
| `-act-delay` | `500ms` | R8: time before the observed state (`ACTS`) follows a command |
| `-act-fault` | none | R8: `stuck` = acknowledge commands (`ACK`) but never move (`ACTS` stays) |
| `-link-timeout` | `10s` | R8: enter the safe state after this long without a valid downlink frame |
| `-in` | see below | R8: where downlink frames come from: `stdin`, `pipe:<path>` or `none` |

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

## Actuator (R8)

With `-actuator`, every frame also carries `ACTS` (the observed state), `ACK` (the last
accepted `CID`) and `SAFE` (1 while in the safe state), and the simulator reads downlink
frames (`FRAME_FORMAT.md` section 7):

| `-out` | Downlink read from (default) |
|---|---|
| `tcp://host:port` | the same connection: the gateway writes into the socket it reads |
| `pipe:<path>` | the named pipe `<path>.down` (created if missing) |
| `stdout` | stdin, so you can type frames by hand |

Behaviour, as a real device would show it:

- it starts in its safe state (`cooling` on, `valve` closed) and leaves it only on a command;
- `ACT` is applied once per `CID` (the last eight are remembered), after `-act-delay`;
- `SAFE:1` takes effect at once; so does the link timeout when no valid downlink frame
  (`ACT`, `SAFE` or `KA`) arrives for `-link-timeout`;
- invalid, stale and repeated frames are dropped whole; with `-hmac` (key from
  `FRAME_HMAC_KEY`) downlink frames must carry a valid tag;
- cold chain: with cooling off, `TEMP` rises by 0.08 °C per sample (up to +15 °C); with
  cooling on it returns towards its steady state.

```bash
go run ./sim/sensor_sim -actuator cooling -period 500ms
C:0;CID:1;ACT:0          # typed: cooling off; ACTS follows after 500 ms, ACK:1
C:1;KA:1                 # keep-alive; without one for 10 s, SAFE:1
```

Tests: `go test ./sim/... ./frame/...` checks every emitted frame of every scenario,
with and without HMAC, against the shared parser, and that the gateway's device side
drops tampered frames, and the actuator's behaviour above (fake clock, and over a real TCP
connection).
