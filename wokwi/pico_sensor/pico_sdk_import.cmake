# Locate the Raspberry Pi Pico SDK from PICO_SDK_PATH (CMake variable or environment).
if (DEFINED ENV{PICO_SDK_PATH} AND (NOT PICO_SDK_PATH))
    set(PICO_SDK_PATH $ENV{PICO_SDK_PATH})
endif ()
if (NOT PICO_SDK_PATH)
    message(FATAL_ERROR "Set PICO_SDK_PATH to your pico-sdk checkout")
endif ()
get_filename_component(PICO_SDK_PATH "${PICO_SDK_PATH}" REALPATH BASE_DIR "${CMAKE_BINARY_DIR}")
include(${PICO_SDK_PATH}/pico_sdk_init.cmake)
