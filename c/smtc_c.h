#pragma once

#if defined(_WIN32)
#if defined(SMTC_EXPORTS)
#define SMTC_EXPORT __declspec(dllexport)
#else
#define SMTC_EXPORT __declspec(dllimport)
#endif
#else
#define SMTC_EXPORT
#endif

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>

SMTC_EXPORT void *smtc_create();
SMTC_EXPORT void smtc_destroy(void *smtc);
SMTC_EXPORT int smtc_init(void *smtc);
SMTC_EXPORT void smtc_update(void *smtc);
SMTC_EXPORT int smtc_retrieve_dirty_data(void *smtc, const wchar_t **artist, const wchar_t **title, const wchar_t **thumbnail_content_type, const uint8_t **thumbnail_data, int *thumbnail_length, int *position, int *duration, int *status);

#ifdef __cplusplus
}
#endif
