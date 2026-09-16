/* C ABI of libzapp_rcodesign: Apple code signing, notarization and stapling
 * without the rcodesign CLI. Built from src/lib.rs; see README.md for the
 * request JSON and the system libraries each target must be linked with. */
#ifndef ZAPP_RCODESIGN_H
#define ZAPP_RCODESIGN_H

#ifdef __cplusplus
extern "C" {
#endif

/* Runs one signing request, given as a NUL-terminated UTF-8 JSON string that
 * must stay valid for the duration of the call. Returns NULL on success, or an
 * owned error string to release with zapp_rcodesign_free exactly once.
 * The call is synchronous and never prompts on stdin. */
char *zapp_rcodesign_run(const char *request);

/* Releases an error returned by zapp_rcodesign_run. NULL is ignored. */
void zapp_rcodesign_free(char *error);

#ifdef __cplusplus
}
#endif

#endif /* ZAPP_RCODESIGN_H */
