add_project(smtc_c
    SHARED smtc.cpp smtc.h smtc_c.cpp smtc_c.h smtc_c.def
    LANGUAGES CXX
    OUTPUT_NAME smtc)

target_compile_definitions(smtc_c PRIVATE SMTC_EXPORTS)
target_link_libraries(smtc_c PRIVATE windowsapp)
set_target_properties(smtc_c PROPERTIES
    ARCHIVE_OUTPUT_NAME smtc_c
)
if(WIN32 AND NOT MSVC)
    target_link_options(SmtcMonitor PRIVATE -municode)
endif()
