add_project(SmtcMonitor EXECUTABLE
    smtc.cpp smtc.h
    main.cpp
    LANGUAGES CXX)
add_library(smtc_c SHARED smtc.cpp smtc.h smtc_c.cpp smtc_c.h smtc_c.def)

# Link against required Windows Runtime libraries
target_link_libraries(SmtcMonitor PRIVATE windowsapp shlwapi "${CMAKE_CURRENT_SOURCE_DIR}/rsrc_windows_amd64.syso")

target_compile_definitions(smtc_c PRIVATE SMTC_EXPORTS)
target_link_libraries(smtc_c PRIVATE windowsapp shlwapi)
set_target_properties(smtc_c PROPERTIES
    OUTPUT_NAME smtc
    ARCHIVE_OUTPUT_NAME smtc_c
    RUNTIME_OUTPUT_DIRECTORY ${CMAKE_SOURCE_DIR}/build/bin/$<0:>
    ARCHIVE_OUTPUT_DIRECTORY ${CMAKE_SOURCE_DIR}/build/lib/$<0:>
    LIBRARY_OUTPUT_DIRECTORY ${CMAKE_SOURCE_DIR}/build/lib/$<0:>
)
if(WIN32 AND NOT MSVC)
    target_link_options(SmtcMonitor PRIVATE -municode)
endif()
