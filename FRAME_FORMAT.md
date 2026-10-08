# FRAME_FORMAT.md — sensor and actuator frame contract

The one contract shared by the three device-side components of the kit. Uplink frames
(sections 1–6) go from the device to the gateway; downlink frames (section 7) carry
actuator commands from the gateway to the device:

| Component | Role | Path |
|---|---|---|
| Wokwi C firmware | uplink producer, downlink consumer | `wokwi/pico_sensor/src/main.c` |
| Sensor simulator | uplink producer, downlink consumer | `sim/sensor_sim/` |
| Gateway | uplink consumer, downlink producer | `gateway/` |

The gateway must not be able to tell simulator from hardware. Anything a producer emits
that this document does not allow is a producer bug; anything the gateway rejects that
this document allows is a gateway bug.

**Version:** 2 (adds the downlink, section 7, and the actuator keys `ACTS`, `ACK`, `SAFE`).
A change to this file is a contract change: update it first, then the
simulator, the Wokwi skeleton and the gateway parser in the same release.

---

## 1. Transport and line framing

- One frame per line: 7-bit ASCII, terminated by `\n` (LF). A `\r` directly before the
  `\n` is tolerated and stripped by the gateway. Producers emit LF only.
- Maximum line length: 128 bytes including the terminator.
- UART (Pico or Wokwi): 115200 baud, 8N1, no flow control.
- Simulator: the same bytes, written to a named pipe or a TCP socket
  (`SERIAL_SOURCE` on the gateway selects which; see `gateway/README.md`).
- No binary data, no escaping, no quoting. Fields never contain `;`, `:` or whitespace.

## 2. Frame grammar

```
frame     = seq-field *( ";" data-field ) [ ";" hmac-field ] LF
seq-field = "S:" 1*5DIGIT                       ; 0..65535
data-field= key ":" value
key       = UPALPHA *7( UPALPHA / DIGIT )       ; 1..8 chars, e.g. TEMP, CO2
value     = [ "-" ] 1*10DIGIT                   ; signed 32-bit integer
hmac-field= "H:" 16LOWHEX                        ; see section 5
```

- The sequence field is always first. At least one data field follows.
- Data fields may appear in any order; a key appears at most once per frame.
- One frame carries every quantity of the scenario sampled at the same instant.
- Default sample period: 1 s from the simulator, 2 s from the Wokwi firmware
  (configurable in both).

| Field | Producer | Consumer | Meaning |
|---|---|---|---|
| `S` | Wokwi C, simulator | gateway | Sequence number, section 4 |
| data keys | Wokwi C (TEMP and BARO from the BMP180, HUM from the DHT22), simulator (every key in section 3) | gateway | Scaled integer reading, section 3 |
| `BOOT` | Wokwi C, simulator | gateway | Producer restart marker, section 4 |
| `H` | Wokwi C, simulator (both only when HMAC is enabled) | gateway | Integrity tag, section 5 |

## 3. Keys and integer scaling

Values are integers: **physical value × scale**, rounded to nearest. No floating point
crosses the wire; the gateway divides by the scale when it builds its JSON service
response and states the unit in its interface contract.

| Key | Quantity | Unit | Scale | Valid range (raw integer) | Scenario |
|---|---|---|---|---|---|
| `TEMP` | Temperature | °C | 100 | −4000 … 8500 | Cold chain, Air quality (Wokwi: BMP180, 0.1 °C resolution) |
| `HUM` | Relative humidity | %RH | 100 | 0 … 10000 | Cold chain (Wokwi: DHT22, 0.1 %RH resolution) |
| `BARO` | Barometric pressure | hPa | 10 | 3000 … 11000 | Any (Wokwi: BMP180, optional) |
| `PWR` | Electrical power | W | 10 | 0 … 1000000 | Building energy |
| `OCC` | Occupancy (presence) | 0 or 1 | 1 | 0 … 1 | Building energy |
| `CO2` | CO₂ concentration | ppm | 1 | 0 … 10000 | Air quality |
| `PRES` | Hydraulic pressure | bar | 100 | 0 … 40000 | Industrial press |
| `CYC` | Press cycle count | cycles | 1 | 0 … 2147483647, monotonic, wraps to 0 | Industrial press |
| `BOOT` | Restart marker | — | 1 | 1 … 65535 (boot epoch) | All, section 4 |
| `ACTS` | Observed actuator state | per actuator | 1 | `cooling`, `valve`: 0 … 1 | With an actuator (R8), section 7 |
| `ACK` | Last accepted downlink `CID`: the device has started acting on it; `ACTS` shows when the actuator has followed | — | 1 | 0 … 2147483647 (0 = none yet) | With an actuator, section 7 |
| `SAFE` | Device is in its safe state | 0 or 1 | 1 | 0 … 1 | With an actuator, section 7 |

Custom scenarios register their keys in the project proposal: same key syntax, integer
scale stated, range stated. A custom key must not reuse a key above with another meaning.

A value outside its valid range is a **range error**: the gateway drops the field (not
the frame), counts it, and logs it once per key per minute.

## 4. Sequence numbers, restart and resync

- `S` starts at 0 after producer start, increments by 1 per frame, and wraps from 65535
  to 0.
- The first frame after start carries a `BOOT` field whose value is a boot epoch: a
  nonzero value the producer picks at start (the simulator uses a random value; the
  Pico uses its ring-oscillator random bits). Example: `S:0;BOOT:40117;TEMP:2210;HUM:5980`.

Gateway rules, with `last` the last accepted sequence number and
`d = (S − last) mod 65536`:

| Case | Condition | Gateway action |
|---|---|---|
| In order | `d == 1` | accept |
| Gap | `2 ≤ d < 32768` | accept; count `d − 1` lost frames |
| Duplicate or stale | `d == 0` or `d ≥ 32768` | drop; count as duplicate |
| Restart | frame carries `BOOT` | accept; reset `last`; log the epoch |
| First frame seen | no `last` yet | accept any `S`; the gateway may have opened the source mid-stream |

Line-level errors (none stop the gateway; all are counted except unknown keys):

| Error | Gateway action |
|---|---|
| Partial line when the source is opened | discard up to and including the first `\n` |
| Line exceeds 128 bytes without `\n` | discard bytes up to and including the next `\n` |
| Non-ASCII byte, empty field, bad key or value syntax | drop the frame |
| Missing or non-first `S` field | drop the frame |
| Unknown key | ignore that field, keep the rest (forward compatibility); collected, not counted |
| Key repeated in one frame | drop the frame |
| Source closed or read error | reopen with back-off (1 s doubling to 30 s); treat the next frame as "first frame seen" |

The gateway exposes the counters (accepted, lost, duplicate, malformed, range errors,
HMAC failures) in its log; they are what a student shows when asked "how do you know
frames are not being lost?".

**Known limitation:** without HMAC, anyone who can write to the pipe, socket or serial
line can inject frames, including a `BOOT` frame that resets sequence checking. With
HMAC, an old authentic `BOOT` frame can still be replayed. Both belong in the STRIDE
analysis.

## 5. Optional HMAC framing (requirement R10 option)

When enabled, every frame ends with an integrity tag:

- Algorithm: HMAC-SHA256, truncated to the first 8 bytes, written as 16 lowercase hex
  digits.
- Key: 32 bytes, pre-shared between producer and gateway, configured as 64 hex digits
  (`FRAME_HMAC_KEY` for the simulator and gateway; a `#define` in the Wokwi firmware).
  Never commit a real key.
- Input: the exact frame bytes from the first byte of `S` up to and including the `;`
  that precedes `H:`. The tag covers the sequence number, so a tag cannot be moved to
  another sequence number.
- Gateway mode (`FRAME_HMAC`): `off` ignores `H`; `optional` verifies `H` when present
  and accepts untagged frames; `required` drops untagged frames and frames with a wrong
  tag. Comparison is constant-time.
- A tag failure is counted and logged with the sequence number; the frame is dropped and
  does not update `last`.

## 6. Worked frames

Plain frames, one per scenario:

| Scenario | Frame | Decoded |
|---|---|---|
| Cold chain | `S:42;TEMP:2247;HUM:6130` | 22.47 °C, 61.30 %RH |
| Building energy | `S:1805;PWR:34215;OCC:1` | 3421.5 W, occupied |
| Air quality | `S:311;CO2:1184;TEMP:2362` | 1184 ppm, 23.62 °C |
| Industrial press | `S:65535;PRES:21750;CYC:48213` | 217.50 bar, cycle 48213; next `S` is 0 |
| Wokwi (BMP180 + DHT22) | `S:7;TEMP:-510;HUM:8800;BARO:10132` | −5.10 °C, 88.00 %RH, 1013.2 hPa |
| Producer restart | `S:0;BOOT:40117;TEMP:2210;HUM:5980` | restart, epoch 40117 |

HMAC frames, test key `000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f`
(for examples and unit tests only):

| Frame | Gateway verdict |
|---|---|
| `S:42;TEMP:2247;HUM:6130;H:6624d8fa493ef251` | accept |
| `S:43;TEMP:2251;HUM:6128;H:851759e6ae5ee27c` | accept |
| `S:43;TEMP:2951;HUM:6128;H:851759e6ae5ee27c` | drop: tag mismatch (TEMP altered; correct tag would be `05552887c6a80e5c`) |
| `S:42;TEMP:2247;HUM:6130;H:6624d8fa493ef251` (again, after 43) | drop: stale sequence (replay) |
| `S:44;TEMP:2249;HUM:6131` with `FRAME_HMAC=required` | drop: untagged |

Reproduce a tag:

```bash
printf 'S:42;TEMP:2247;HUM:6130;' | openssl dgst -sha256 \
  -mac HMAC -macopt hexkey:000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f \
  | awk '{print substr($NF,1,16)}'
# 6624d8fa493ef251
```

Error cases:

| Input line | Gateway action |
|---|---|
| `TEMP:2247;HUM:6130` | drop: no `S` field |
| `S:45;TEMP:22.47` | drop: value is not an integer |
| `S:46;TEMP:2247;TEMP:2250` | drop: repeated key |
| `S:47;TEMP:2247;LUX:880` | accept TEMP; ignore unknown `LUX` |
| `S:48;HUM:12000` | accept frame; drop `HUM` field as range error |
| `S:52;TEMP:2250` after `S:48` | accept; 3 lost frames |

## 7. Downlink: actuator commands (requirement R8)

Downlink frames carry commands from the gateway to the device, over the same link as the
uplink: the same UART or serial device (read and write), the same TCP connection, or, when
the uplink is a named pipe, a second named pipe `<pipe>.down`. Transport and line framing
are as in section 1: one 7-bit ASCII line, LF-terminated, at most 128 bytes.

```
dframe     = cseq-field *( ";" cmd-field ) [ ";" hmac-field ] LF
cseq-field = "C:" 1*5DIGIT                       ; 0..65535, its own counter
cmd-field  = key ":" value                       ; the key and value syntax of section 2
hmac-field = "H:" 16LOWHEX                        ; as section 5
```

| Key | Meaning | Values |
|---|---|---|
| `C` | downlink sequence number, always first | 0 … 65535; rules as section 4 |
| `BOOT` | first downlink after a gateway start | 1 … 65535 (gateway boot epoch) |
| `CID` | command ID (idempotency) | 1 … 2147483647 |
| `ACT` | commanded actuator state | `cooling`, `valve`: 0 … 1 |
| `SAFE` | enter the safe state now | 1 |
| `KA` | keep-alive: no command, the gateway is alive | 1 |

A downlink frame is exactly one of these (`BOOT` may be added to any of them):

| Kind | Frame | Device action |
|---|---|---|
| Command | `C:<n>;CID:<id>;ACT:<v>` | drive the actuator to `ACT` once for this `CID`; report `ACK` from the next frame, `ACTS` when the actuator has followed |
| Safe | `C:<n>;CID:<id>;SAFE:1` | enter the safe state once for this `CID`, then report `SAFE:1` and `ACK` |
| Keep-alive | `C:<n>;KA:1` | nothing, except that the link counts as alive |

Device rules:

- **Sequence:** as section 4, with `C` in place of `S`: in order or gap: accept; duplicate
  or stale: drop and count; `BOOT`: accept and reset; first frame seen: accept any `C`.
- **Idempotency:** the device keeps the last 8 applied `CID` values. A frame with one of
  them is not applied again; `ACK` keeps reporting it.
- **Validity:** a command or safe frame is dropped (and counted) if it has no `CID`, has both
  `ACT` and `SAFE`, or has an `ACT` value outside the actuator's range. Unlike an uplink
  range error, a bad command is never partly applied. Unknown keys are ignored.
- **Link timeout:** if no valid downlink frame (command, safe or keep-alive) arrives for
  `LINK_TIMEOUT` (default 10 s), the device enters its safe state by itself. The gateway
  therefore sends a keep-alive at least every `LINK_TIMEOUT` / 3 when it has no command.
- **Safe state:** the actuator's documented safe value (the simulator: `cooling` → 1, cooling
  on, so the goods stay cold; `valve` → 0, closed) and `SAFE:1` in every uplink frame. The
  device starts in its safe state (no gateway has spoken yet) and leaves it only on a new
  command frame. Whether a command may follow
  an emergency stop is the gateway's decision, not the device's.
- **Feedback:** with an actuator, every uplink frame carries `ACTS`, `ACK` and `SAFE`.
  `ACTS` is the observed state, which can lag behind `ACT` or differ from it (an actuator
  that sticks).
- **HMAC:** the same algorithm and **the same key** as section 5. The tag input runs from
  `C:` up to and including the `;` before `H:`. Uplink input starts at `S:`, so a tag
  cannot be replayed into the other direction. When the device tags its uplink, it requires
  tagged downlink frames and drops untagged ones; otherwise it accepts untagged frames and
  verifies a tag when present.

Worked downlink:

| Frame | Device verdict |
|---|---|
| `C:0;BOOT:211;KA:1` | accept; gateway restart, epoch 211 |
| `C:12;CID:907;ACT:1` | accept; cooling on, `ACK:907` |
| `C:12;CID:907;ACT:1` (again) | drop: stale sequence |
| `C:13;CID:907;ACT:1` | accept; `CID` already applied, nothing done |
| `C:14;CID:908;SAFE:1` | accept; safe state, `SAFE:1`, `ACK:908` |
| `C:15;ACT:1` | drop: no `CID` |
| `C:16;CID:909;ACT:1;SAFE:1` | drop: both `ACT` and `SAFE` |
| `C:17;CID:910;ACT:5` | drop: `ACT` out of range |
| `C:12;CID:907;ACT:1;H:354a9ceaa3cd5bba` | accept (tag correct) |
| `C:12;CID:907;ACT:0;H:354a9ceaa3cd5bba` | drop: tag mismatch (`ACT` altered; correct tag would be `bd6a536697f3341b`) |

The matching uplink after the command: `S:431;TEMP:412;HUM:6020;ACTS:1;ACK:907;SAFE:0`.

Reproduce a downlink tag:

```bash
printf 'C:12;CID:907;ACT:1;' | openssl dgst -sha256 \
  -mac HMAC -macopt hexkey:000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f \
  | awk '{print substr($NF,1,16)}'
# 354a9ceaa3cd5bba
```

**Known limitation:** without HMAC, anyone who can write to the link can command the
actuator; with HMAC, an old authentic `BOOT` downlink can still be replayed to reset the
device's sequence check. The gateway is the only party that may command the device: who
may command the gateway, and in which order commands run, is the gateway's own logic (R8).
