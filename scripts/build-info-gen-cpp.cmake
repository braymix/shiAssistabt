# Regenerated build-info generator for prima.cpp.
#
# prima.cpp's common/CMakeLists.txt runs this via `cmake -P` to turn
# common/build-info.cpp.in into common/build-info.cpp, but the upstream fork
# dropped this file from the repo (leaving only the template), which breaks the
# build with "Not a file: .../common/cmake/build-info-gen-cpp.cmake". shikA's
# engine builds drop this replacement back in before compiling.
#
# The invoking custom command passes -DMSVC, -DCMAKE_C_COMPILER_VERSION,
# -DCMAKE_C_COMPILER_ID and -DCMAKE_VS_PLATFORM_NAME. We derive git build info
# best-effort and configure the template. Paths are resolved from this file's
# own location so the working directory does not matter.

set(BUILD_NUMBER 0)
set(BUILD_COMMIT "unknown")
set(BUILD_COMPILER "unknown")
set(BUILD_TARGET "unknown")

find_program(GIT_EXE NAMES git)
if(GIT_EXE)
    execute_process(
        COMMAND ${GIT_EXE} rev-list --count HEAD
        WORKING_DIRECTORY ${CMAKE_CURRENT_LIST_DIR}
        OUTPUT_VARIABLE _n OUTPUT_STRIP_TRAILING_WHITESPACE
        ERROR_QUIET RESULT_VARIABLE _rc)
    if(_rc EQUAL 0 AND _n)
        set(BUILD_NUMBER ${_n})
    endif()
    execute_process(
        COMMAND ${GIT_EXE} rev-parse --short HEAD
        WORKING_DIRECTORY ${CMAKE_CURRENT_LIST_DIR}
        OUTPUT_VARIABLE _sha OUTPUT_STRIP_TRAILING_WHITESPACE
        ERROR_QUIET RESULT_VARIABLE _rc2)
    if(_rc2 EQUAL 0 AND _sha)
        set(BUILD_COMMIT ${_sha})
    endif()
endif()

if(MSVC)
    set(BUILD_COMPILER "MSVC ${CMAKE_C_COMPILER_VERSION}")
    if(CMAKE_VS_PLATFORM_NAME)
        set(BUILD_TARGET "${CMAKE_VS_PLATFORM_NAME}")
    endif()
else()
    set(BUILD_COMPILER "${CMAKE_C_COMPILER_ID} ${CMAKE_C_COMPILER_VERSION}")
endif()

configure_file(
    "${CMAKE_CURRENT_LIST_DIR}/../build-info.cpp.in"
    "${CMAKE_CURRENT_LIST_DIR}/../build-info.cpp"
    @ONLY)
