cmake_minimum_required(VERSION 3.22)
project(VitAuditionPreviewTests LANGUAGES CXX)

enable_testing()

add_executable(VitAuditionPreviewStateTests
    AuditionPreviewStateTests.cpp
    ../Source/Service/AuditionPreviewState.cpp)

target_compile_features(VitAuditionPreviewStateTests PRIVATE cxx_std_20)
target_include_directories(VitAuditionPreviewStateTests PRIVATE
    ../Source/Service)

if(MSVC)
    target_compile_options(VitAuditionPreviewStateTests PRIVATE /W4 /permissive-)
else()
    target_compile_options(VitAuditionPreviewStateTests PRIVATE -Wall -Wextra -Wpedantic)
endif()

add_test(NAME VitAuditionPreviewStateTests COMMAND VitAuditionPreviewStateTests)
