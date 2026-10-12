#include <windows.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <fcntl.h>
#include "../diagnostic_queue.h"
#define CHECK(value) do { if (!(value)) { fprintf(stderr, "FAIL line %d: %s\n", __LINE__, #value); return 1; } } while (0)
int main(void) {
    HANDLE reader, writer;
    CHECK(CreatePipe(&reader, &writer, NULL, 4096));
    int saved = _dup(_fileno(stderr));
    int redirected = _open_osfhandle((intptr_t)writer, _O_WRONLY | _O_BINARY);
    CHECK(saved >= 0 && redirected >= 0);
    CHECK(_dup2(redirected, _fileno(stderr)) == 0);
    CHECK(diagnostic_queue_start());
    ULONGLONG started = GetTickCount64();
    for (int i = 0; i < 20000; i++) diagnostic_enqueue("diagnostic engine=Blocked output test %d", i);
    BOOL fast = GetTickCount64() - started < 1000;
    AcquireSRWLockShared(&diagnostic_lock);
    BOOL bounded = diagnostic_count <= DIAGNOSTIC_QUEUE_LINES && diagnostic_dropped > 0;
    ReleaseSRWLockShared(&diagnostic_lock);
    started = GetTickCount64();
    diagnostic_queue_stop();
    BOOL stopped = GetTickCount64() - started < 3000;
    _dup2(saved, _fileno(stderr)); _close(saved); _close(redirected); CloseHandle(reader);
    CHECK(fast && bounded && stopped);
    CHECK(diagnostic_queue_start());
    diagnostic_enqueue("diagnostic engine=Restarted writer");
    CHECK(diagnostic_queue_flush(3000));
    diagnostic_queue_stop();
    puts("PASS: blocked diagnostic pipe does not stall producers; queue bounds and cancellation work");
    return 0;
}
