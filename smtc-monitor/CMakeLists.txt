add_project(SmtcMonitor EXECUTABLE
    smtc.cpp smtc.h
    main.cpp
    LANGUAGES CXX)
# Link against required Windows Runtime libraries
target_link_libraries(SmtcMonitor PRIVATE windowsapp "${CMAKE_CURRENT_SOURCE_DIR}/rsrc_windows_amd64.syso")
if(WIN32 AND NOT MSVC)
    target_link_options(SmtcMonitor PRIVATE -municode)
endif()
