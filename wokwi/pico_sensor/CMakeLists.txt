cmake_minimum_required(VERSION 3.13)

# Requires the Raspberry Pi Pico SDK: set PICO_SDK_PATH.
include(pico_sdk_import.cmake)

project(pico_sensor C CXX ASM)
set(CMAKE_C_STANDARD 11)
pico_sdk_init()

add_executable(pico_sensor src/main.c)
target_link_libraries(pico_sensor pico_stdlib hardware_i2c hardware_uart)

# Frames go out on UART0 directly; keep stdio off both UART and USB.
pico_enable_stdio_uart(pico_sensor 0)
pico_enable_stdio_usb(pico_sensor 0)

pico_add_extra_outputs(pico_sensor)
