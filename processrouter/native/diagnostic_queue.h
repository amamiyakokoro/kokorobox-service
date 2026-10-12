#ifndef KOKORO_DIAGNOSTIC_QUEUE_H
#define KOKORO_DIAGNOSTIC_QUEUE_H
#include <io.h>
#include <stdarg.h>

#define DIAGNOSTIC_QUEUE_LINES 1024
#define DIAGNOSTIC_LINE_SIZE 2048
static SRWLOCK diagnostic_lock = SRWLOCK_INIT;
static char diagnostic_lines[DIAGNOSTIC_QUEUE_LINES][DIAGNOSTIC_LINE_SIZE];
static unsigned diagnostic_head, diagnostic_count;
static unsigned long diagnostic_dropped;
static BOOL diagnostic_quitting;
static HANDLE diagnostic_event, diagnostic_idle, diagnostic_thread;

// Engine callbacks only copy a bounded line; no console or pipe I/O runs here.
static void diagnostic_enqueue(const char *format, ...) {
    char line[DIAGNOSTIC_LINE_SIZE];
    va_list arguments;
    va_start(arguments, format);
    vsnprintf(line, sizeof(line) - 1, format, arguments);
    va_end(arguments);
    line[sizeof(line) - 2] = '\0';
    size_t length = strlen(line);
    if (length == 0 || line[length - 1] != '\n') { line[length++] = '\n'; line[length] = '\0'; }
    AcquireSRWLockExclusive(&diagnostic_lock);
    if (diagnostic_event && !diagnostic_quitting) {
        if (diagnostic_count < DIAGNOSTIC_QUEUE_LINES) {
            strcpy_s(diagnostic_lines[(diagnostic_head + diagnostic_count) % DIAGNOSTIC_QUEUE_LINES], DIAGNOSTIC_LINE_SIZE, line);
            diagnostic_count++;
        } else diagnostic_dropped++;
        ResetEvent(diagnostic_idle);
        SetEvent(diagnostic_event);
    }
    ReleaseSRWLockExclusive(&diagnostic_lock);
}

static BOOL diagnostic_write(const char *line) {
    HANDLE output = (HANDLE)_get_osfhandle(_fileno(stderr));
    size_t length = strlen(line), offset = 0;
    while (offset < length) {
        AcquireSRWLockShared(&diagnostic_lock);
        BOOL quitting = diagnostic_quitting;
        ReleaseSRWLockShared(&diagnostic_lock);
        if (quitting) return FALSE;
        DWORD written = 0;
        if (!WriteFile(output, line + offset, (DWORD)(length - offset), &written, NULL) || !written) return FALSE;
        offset += written;
    }
    return TRUE;
}

static DWORD WINAPI diagnostic_writer(LPVOID argument) {
    (void)argument;
    for (;;) {
        WaitForSingleObject(diagnostic_event, INFINITE);
        for (;;) {
            char line[DIAGNOSTIC_LINE_SIZE];
            unsigned long dropped;
            AcquireSRWLockExclusive(&diagnostic_lock);
            if (diagnostic_quitting) { ReleaseSRWLockExclusive(&diagnostic_lock); return 0; }
            if (!diagnostic_count && !diagnostic_dropped) {
                SetEvent(diagnostic_idle);
                ReleaseSRWLockExclusive(&diagnostic_lock);
                break;
            }
            dropped = diagnostic_dropped; diagnostic_dropped = 0;
            line[0] = '\0';
            if (diagnostic_count) {
                strcpy_s(line, sizeof(line), diagnostic_lines[diagnostic_head]);
                diagnostic_head = (diagnostic_head + 1) % DIAGNOSTIC_QUEUE_LINES;
                diagnostic_count--;
            }
            ReleaseSRWLockExclusive(&diagnostic_lock);
            if (line[0]) diagnostic_write(line);
            if (dropped) {
                char note[128];
                snprintf(note, sizeof(note), "diagnostic engine=level=warn Dropped %lu diagnostic lines (output is too slow)\n", dropped);
                diagnostic_write(note);
            }
        }
    }
}

static BOOL diagnostic_queue_start(void) {
    if (diagnostic_thread) return TRUE;
    diagnostic_head = diagnostic_count = diagnostic_dropped = 0;
    diagnostic_quitting = FALSE;
    diagnostic_event = CreateEventW(NULL, FALSE, FALSE, NULL);
    diagnostic_idle = CreateEventW(NULL, TRUE, TRUE, NULL);
    if (diagnostic_event && diagnostic_idle)
        diagnostic_thread = CreateThread(NULL, 0, diagnostic_writer, NULL, 0, NULL);
    if (diagnostic_thread) return TRUE;
    if (diagnostic_event) CloseHandle(diagnostic_event);
    if (diagnostic_idle) CloseHandle(diagnostic_idle);
    diagnostic_event = diagnostic_idle = NULL;
    return FALSE;
}

static BOOL diagnostic_queue_flush(DWORD timeout) {
    return !diagnostic_idle || WaitForSingleObject(diagnostic_idle, timeout) == WAIT_OBJECT_0;
}

static void diagnostic_queue_stop(void) {
    if (!diagnostic_thread) return;
    diagnostic_queue_flush(1000);
    AcquireSRWLockExclusive(&diagnostic_lock);
    diagnostic_quitting = TRUE;
    SetEvent(diagnostic_event);
    ReleaseSRWLockExclusive(&diagnostic_lock);
    // Retry cancellation to cover the race between checking quitting and WriteFile.
    while (WaitForSingleObject(diagnostic_thread, 50) == WAIT_TIMEOUT)
        CancelSynchronousIo(diagnostic_thread);
    AcquireSRWLockExclusive(&diagnostic_lock);
    CloseHandle(diagnostic_thread); CloseHandle(diagnostic_event); CloseHandle(diagnostic_idle);
    diagnostic_thread = diagnostic_event = diagnostic_idle = NULL;
    diagnostic_count = diagnostic_dropped = 0;
    ReleaseSRWLockExclusive(&diagnostic_lock);
}
#endif
