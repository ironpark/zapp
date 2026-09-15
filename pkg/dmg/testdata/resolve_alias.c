// macOS-only test oracle. Production image generation does not use Carbon.
#include <CoreServices/CoreServices.h>
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char **argv) {
    if (argc != 2) return 2;
    FILE *file = fopen(argv[1], "rb");
    if (!file) return 2;
    fseek(file, 0, SEEK_END);
    long size = ftell(file);
    rewind(file);
    AliasHandle alias = (AliasHandle)NewHandle(size);
    if (!alias || fread(*alias, 1, size, file) != (size_t)size) return 2;
    fclose(file);
    FSRef target;
    Boolean changed = false;
    OSStatus status = FSResolveAliasWithMountFlags(NULL, alias, &target, &changed,
                                                  kResolveAliasFileNoUI);
    DisposeHandle((Handle)alias);
    if (status) { fprintf(stderr, "resolve alias: %d\n", (int)status); return 1; }
    UInt8 path[4096];
    status = FSRefMakePath(&target, path, sizeof(path));
    if (status) return 1;
    puts((char *)path);
    return 0;
}
