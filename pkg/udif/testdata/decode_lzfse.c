// Test-only oracle for the streaming API used by macOS DiskImages.
#include <compression.h>
#include <stdio.h>
#include <stdlib.h>

int main(void) {
    const size_t capacity = 4 << 20;
    void *input = malloc(capacity);
    void *output = malloc(capacity);
    if (!input || !output) return 1;
    size_t count = fread(input, 1, capacity, stdin);
    if (ferror(stdin) || !feof(stdin)) return 1;

    compression_stream stream = {0};
    if (compression_stream_init(&stream, COMPRESSION_STREAM_DECODE,
                                COMPRESSION_LZFSE) != COMPRESSION_STATUS_OK)
        return 1;
    stream.src_ptr = input;
    stream.src_size = count;
    stream.dst_ptr = output;
    stream.dst_size = capacity;
    compression_status status = compression_stream_process(
        &stream, COMPRESSION_STREAM_FINALIZE);
    size_t decoded = capacity - stream.dst_size;
    int failed = status != COMPRESSION_STATUS_END || stream.src_size != 0;
    if (failed)
        fprintf(stderr, "status %d, remaining input %zu, output %zu\n",
                status, stream.src_size, decoded);
    if (fwrite(output, 1, decoded, stdout) != decoded) failed = 1;
    compression_stream_destroy(&stream);
    free(input);
    free(output);
    return failed;
}
