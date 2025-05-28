add_project(SmtcMonitor EXECUTABLE
    smtc.cpp smtc.h
    main.cpp
    resource.rc
    LANGUAGES CXX)
# Link against required Windows Runtime libraries
target_link_libraries(SmtcMonitor PRIVATE windowsapp)
if(WIN32 AND NOT MSVC)
    target_link_options(SmtcMonitor PRIVATE -municode)
endif()
