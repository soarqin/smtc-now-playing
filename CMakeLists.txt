cmake_minimum_required(VERSION 3.15)
cmake_policy(SET CMP0091 NEW)
if(CMAKE_VERSION VERSION_GREATER_EQUAL 3.24)
    cmake_policy(SET CMP0135 NEW)
endif()

list(APPEND CMAKE_MODULE_PATH "${CMAKE_SOURCE_DIR}/cmake")
include(CustomCompilerOptions)
fix_compile_flags()
fix_release_flags(STRIP_BINARY ON USE_LTO ON USE_STATIC_CRT ON)

include(GlobalOptions)
add_global_options(C_STANDARD 17 CXX_STANDARD 20)

include(ProjectMacros)

project(SmtcNowPlaying)

add_subdirectory(smtc_c)
