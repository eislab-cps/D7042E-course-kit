// pico_sensor — D7042E device skeleton (Raspberry Pi Pico, C, Pico SDK).
//
// Hardware (same in Wokwi and on the optional real kit, see diagram.json):
//   BMP180  I2C0  SDA=GP4 SCL=GP5  address 0x77  -> TEMP, BARO
//   DHT22   single-wire data on GP15             -> HUM
//   LED     GP16 through 220 ohm                 -> optional actuation (R8)
//   UART0   TX=GP0 RX=GP1  115200 8N1            -> frames to the gateway
//
// Output contract: FRAME_FORMAT.md (one line per sample, "S:<seq>;KEY:VALUE...\n",
// integers = physical value x scale). This skeleton only initialises the peripherals
// and sends the restart frame. Everything marked TODO is your part of the assignment.

#include <stdio.h>
#include "pico/stdlib.h"
#include "hardware/i2c.h"
#include "hardware/uart.h"
#include "hardware/gpio.h"
#include "hardware/structs/rosc.h"

#define UART_ID       uart0
#define UART_BAUD     115200
#define UART_TX_PIN   0
#define UART_RX_PIN   1

#define I2C_PORT      i2c0
#define I2C_SDA_PIN   4
#define I2C_SCL_PIN   5
#define I2C_BAUD      (100 * 1000)
#define BMP180_ADDR   0x77

#define DHT22_PIN     15
#define LED_PIN       16

#define SAMPLE_PERIOD_MS 2000   // FRAME_FORMAT.md section 2: Wokwi default 2 s

static void uart_setup(void) {
    uart_init(UART_ID, UART_BAUD);
    gpio_set_function(UART_TX_PIN, GPIO_FUNC_UART);
    gpio_set_function(UART_RX_PIN, GPIO_FUNC_UART);
    uart_set_format(UART_ID, 8, 1, UART_PARITY_NONE);
}

static void i2c_setup(void) {
    i2c_init(I2C_PORT, I2C_BAUD);
    gpio_set_function(I2C_SDA_PIN, GPIO_FUNC_I2C);
    gpio_set_function(I2C_SCL_PIN, GPIO_FUNC_I2C);
    gpio_pull_up(I2C_SDA_PIN);
    gpio_pull_up(I2C_SCL_PIN);
}

static void gpio_setup(void) {
    gpio_init(DHT22_PIN);            // single-wire: idle high, driven by both sides
    gpio_set_dir(DHT22_PIN, GPIO_IN);
    gpio_pull_up(DHT22_PIN);
    gpio_init(LED_PIN);
    gpio_set_dir(LED_PIN, GPIO_OUT);
    gpio_put(LED_PIN, 0);
}

// Boot epoch for the BOOT field (FRAME_FORMAT.md section 4): 16 random bits from the
// ring oscillator, never 0.
static uint16_t boot_epoch(void) {
    uint16_t v = 0;
    for (int i = 0; i < 16; i++) {
        v = (uint16_t)((v << 1) | (rosc_hw->randombit & 1u));
        busy_wait_us(10);
    }
    return v ? v : 1;
}

int main(void) {
    uart_setup();
    i2c_setup();
    gpio_setup();

    uint16_t seq = 0;
    char line[128];   // FRAME_FORMAT.md: at most 128 bytes including the LF

    // Restart frame: sequence 0 with the boot epoch. A complete frame also carries the
    // readings (e.g. "S:0;BOOT:40117;TEMP:2210;HUM:5980").
    snprintf(line, sizeof line, "S:%u;BOOT:%u\n", seq, boot_epoch());
    uart_puts(UART_ID, line);
    seq++;

    // TODO: read the BMP180 calibration registers (0xAA..0xBF) once.

    while (true) {
        sleep_ms(SAMPLE_PERIOD_MS);

        // TODO: BMP180 — start a temperature and a pressure conversion over I2C,
        //       read the raw values and apply the datasheet compensation formula.
        //       TEMP = degrees C x 100, BARO = hPa x 10.
        // TODO: DHT22 — send the start pulse on DHT22_PIN, read the 40 data bits by
        //       timing the pulses, check the checksum. HUM = %RH x 100.
        // TODO: build "S:<seq>;TEMP:<t>;HUM:<h>;BARO:<p>\n" into line, write it with
        //       uart_puts(UART_ID, line) and increment seq (it wraps at 65535).
        // TODO (R10 option B): append ";H:<tag>" per FRAME_FORMAT.md section 5.
        (void)seq;
    }
}
