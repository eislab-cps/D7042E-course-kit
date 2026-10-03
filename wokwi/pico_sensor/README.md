# wokwi/pico_sensor — device skeleton

Raspberry Pi Pico in C (Pico SDK) with a BMP180 (I2C: temperature, pressure), a DHT22
(single-wire: humidity) and an LED. The same wiring and code run in the Wokwi browser
simulator and on the optional real parts. Output: `FRAME_FORMAT.md` frames on UART0.

| Signal | Pico pin |
|---|---|
| UART0 TX / RX (frames, 115200 8N1) | GP0 / GP1 |
| BMP180 SDA / SCL (I2C0, address 0x77) | GP4 / GP5 |
| DHT22 data | GP15 |
| LED (through 220 Ω) | GP16 |

The skeleton initialises UART, I2C and the GPIOs and sends the restart frame
(`S:0;BOOT:<epoch>`). The sensor reads, the frame loop and the optional HMAC tag are
marked `TODO` in `src/main.c`.

## Run in the browser (wokwi.com)

1. Open https://wokwi.com/projects/new/pi-pico-sdk.
2. Replace `main.c` with the contents of `src/main.c` and `diagram.json` with this
   folder's `diagram.json`.
3. Start the simulation. The serial monitor shows `S:0;BOOT:<number>`. Click the BMP180
   or the DHT22 to change their values while it runs.

## Build locally

```bash
export PICO_SDK_PATH=/path/to/pico-sdk        # needs arm-none-eabi-gcc and cmake
cmake -S . -B build && cmake --build build     # -> build/pico_sensor.uf2 and .elf
```

`wokwi.toml` points the Wokwi VS Code extension at `build/`. On real hardware, copy the
`.uf2` to the Pico in BOOTSEL mode and read UART0 with a USB–serial adapter on GP0/GP1
(the gateway's `SERIAL_SOURCE=/dev/ttyUSB0`).
