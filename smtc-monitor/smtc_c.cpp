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

int smtc_retrieve_dirty_data(void *smtc, wchar_t *artist, wchar_t *title, wchar_t *thumbnail_path, int *position, int *duration, int *status) {
    return static_cast<Smtc*>(smtc)->retrieveDirtyData(artist, title, thumbnail_path, position, duration, status);
}

}
