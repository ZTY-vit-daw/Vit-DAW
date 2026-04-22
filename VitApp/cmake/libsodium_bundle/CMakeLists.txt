cmake_minimum_required(VERSION 3.22)

project(libsodium_bundle LANGUAGES C)

if (NOT DEFINED VIT_LIBSODIUM_ROOT)
    message(FATAL_ERROR "VIT_LIBSODIUM_ROOT must be set to the extracted libsodium-x.y.z tree")
endif()

file(GLOB_RECURSE SODIUM_C_SOURCES CONFIGURE_DEPENDS
    "${VIT_LIBSODIUM_ROOT}/src/libsodium/*.c")

add_library(sodium STATIC ${SODIUM_C_SOURCES})

target_include_directories(sodium
    PUBLIC
        "${VIT_LIBSODIUM_ROOT}/src/libsodium/include"
        "${VIT_LIBSODIUM_ROOT}/src/libsodium/include/sodium"
        "${CMAKE_CURRENT_BINARY_DIR}/include"
    PRIVATE
        "${VIT_LIBSODIUM_ROOT}/src/libsodium/include/sodium"
        "${VIT_LIBSODIUM_ROOT}/src/libsodium/include/sodium/private"
        "${CMAKE_CURRENT_BINARY_DIR}/include/sodium")

set(VERSION 1.0.20)
set(SODIUM_LIBRARY_VERSION_MAJOR 26)
set(SODIUM_LIBRARY_VERSION_MINOR 2)
set(SODIUM_LIBRARY_MINIMAL_DEF "")

configure_file(
    "${VIT_LIBSODIUM_ROOT}/src/libsodium/include/sodium/version.h.in"
    "${CMAKE_CURRENT_BINARY_DIR}/include/sodium/version.h"
    @ONLY)

target_compile_definitions(sodium
    PUBLIC
        SODIUM_STATIC
    PRIVATE
        CONFIGURED
        $<$<C_COMPILER_ID:MSVC>:_CRT_SECURE_NO_WARNINGS>)

if (CMAKE_C_COMPILER_ID STREQUAL "Clang" AND CMAKE_C_COMPILER_FRONTEND_VARIANT STREQUAL "MSVC")
    target_compile_options(sodium PRIVATE -mavx2 -maes -mpclmul -mssse3)
endif()
