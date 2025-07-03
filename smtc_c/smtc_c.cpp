#include "smtc_c.h"

#include "smtc.h"

extern "C" {

void *smtc_create() {
    return new Smtc();
}

void smtc_destroy(void *smtc) {
    delete static_cast<Smtc*>(smtc);
}

int smtc_init(void *smtc) {
    return static_cast<Smtc*>(smtc)->init();
}

void smtc_update(void *smtc) {
    static_cast<Smtc*>(smtc)->update();
}

int smtc_retrieve_dirty_data(void *smtc, const wchar_t **artist, const wchar_t **title, const wchar_t **thumbnail_content_type, const uint8_t **thumbnail_data, int *thumbnail_length, int *position, int *duration, int *status) {
    return static_cast<Smtc*>(smtc)->retrieveDirtyData(artist, title, thumbnail_content_type, thumbnail_data, thumbnail_length, position, duration, status);
}

}
