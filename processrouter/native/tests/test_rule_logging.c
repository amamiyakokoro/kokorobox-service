#define PROXYBRIDGE_EXPORTS
#define main router_program_main
#include "../kokorobox_process_router.c"
#undef main
#include <io.h>

typedef struct FakeRule {
    BOOL active;
    BOOL enabled;
    RuleAction action;
    RuleProtocol protocol;
    char process[2048];
} FakeRule;

static FakeRule installed[64];
static UINT32 next_id = 1;
static LogCallback fake_logger;
static BOOL fail_commit;
static BOOL fail_start;
static char captured[16384];

#define CHECK(condition) do { if (!(condition)) { \
    fprintf(stderr, "FAIL line %d: %s\n", __LINE__, #condition); return 1; \
} } while (0)

UINT32 ProxyBridge_AddRule(const char *process, const char *hosts, const char *ports,
                          const char *domains, RuleProtocol protocol, RuleAction action, UINT32 proxy) {
    UINT32 id = next_id++;
    char message[256];
    if (id >= 64) return 0;
    installed[id].active = installed[id].enabled = TRUE;
    installed[id].action = action;
    installed[id].protocol = protocol;
    strcpy_s(installed[id].process, sizeof(installed[id].process), process);
    _snprintf_s(message, sizeof(message), _TRUNCATE, "Added rule ID: %u for process '%s' (Action: %d)", id, process, action);
    if (fake_logger) fake_logger(message);
    return id;
}
BOOL ProxyBridge_DeleteRule(UINT32 id) {
    char message[80];
    if (fail_commit && id == guard_id) return FALSE;
    installed[id].active = FALSE;
    _snprintf_s(message, sizeof(message), _TRUNCATE, "Deleted rule ID: %u", id);
    if (fake_logger) fake_logger(message);
    return TRUE;
}
BOOL ProxyBridge_DisableRule(UINT32 id) { installed[id].enabled = FALSE; return TRUE; }
BOOL ProxyBridge_EditRule(UINT32 id, const char *process, const char *hosts, const char *ports,
                         const char *domains, RuleProtocol protocol, RuleAction action, UINT32 proxy) { return TRUE; }
BOOL ProxyBridge_MoveRuleToPosition(UINT32 id, UINT32 position) { return TRUE; }
UINT32 ProxyBridge_AddProxyConfig(ProxyType type, const char *host, UINT16 port,
                                 const char *username, const char *password, BOOL resolve) { return 1; }
BOOL ProxyBridge_DeleteProxyConfig(UINT32 id) { return TRUE; }
void ProxyBridge_SetLogCallback(LogCallback callback) { fake_logger = callback; }
void ProxyBridge_SetConnectionCallback(ConnectionCallback callback) {}
void ProxyBridge_SetLocalhostViaProxy(BOOL enabled) {}
void ProxyBridge_SetLoopbackBypassEnabled(BOOL enabled) {}
void ProxyBridge_SetProxyUdpDnsEnabled(BOOL enabled) {}
void ProxyBridge_SetFailClosedOnUnknownOwner(BOOL enabled) {}
void ProxyBridge_SetTrafficLoggingEnabled(BOOL enabled) {}
BOOL ProxyBridge_Start(void) {
    if (fake_logger) fake_logger("Rule: C:\\Game\\game.exe -> BLOCK");
    return !fail_start;
}
BOOL ProxyBridge_Stop(void) { return TRUE; }

static BOOL replace_and_capture(const char *rules, BOOL logging, BOOL *replaced) {
    char command[8192];
    FILE *capture = tmpfile();
    int saved;
    size_t length;
    if (!capture) return FALSE;
    fflush(stderr);
    saved = _dup(_fileno(stderr));
    if (saved == -1 || _dup2(_fileno(capture), _fileno(stderr)) != 0) {
        if (saved != -1) _close(saved);
        fclose(capture);
        return FALSE;
    }
    _snprintf_s(command, sizeof(command), _TRUNCATE,
        "{\"version\":1,\"command\":\"replace_rules\",\"proxy\":{\"host\":\"127.0.0.1\",\"port\":7891},"
        "\"failClosed\":true,\"proxyUdpDns\":false,\"diagnosticLogging\":%s,\"rules\":[%s]}",
        logging ? "true" : "false", rules);
    *replaced = replace_rules(command);
    fflush(stderr);
    _dup2(saved, _fileno(stderr));
    _close(saved);
    rewind(capture);
    length = fread(captured, 1, sizeof(captured) - 1, capture);
    captured[length] = '\0';
    fclose(capture);
    return TRUE;
}

int main(void) {
    const char *proxy_rule = "{\"processPattern\":\"C:\\\\Game\\\\game.exe\",\"protocol\":\"BOTH\",\"action\":\"PROXY\",\"enabled\":true,\"priority\":1}";
    const char *blocked_rule = "{\"processPattern\":\"C:\\\\Game\\\\game.exe\",\"protocol\":\"TCP\",\"action\":\"BLOCK\",\"enabled\":true,\"priority\":1}";
    const char *with_disabled = "{\"processPattern\":\"game.exe\",\"protocol\":\"UDP\",\"action\":\"DIRECT\",\"enabled\":true,\"priority\":1},"
        "{\"processPattern\":\"disabled.exe\",\"protocol\":\"BOTH\",\"action\":\"PROXY\",\"enabled\":false,\"priority\":2}";
    BOOL replaced;
    const char *removed;
    const char *summary;

    CHECK(replace_and_capture(proxy_rule, TRUE, &replaced) && replaced);
    removed = strstr(captured, "Temporary BLOCK guard removed");
    summary = strstr(captured, "Active routing rules committed: applications=1 exclusions=2");
    CHECK(removed && summary && removed < summary);
    CHECK(strstr(captured, "level=debug Rebuilding routing rules"));
    CHECK(strstr(captured, "level=debug Pending rule: C:\\Game\\game.exe -> BLOCK"));
    CHECK(strstr(summary, "process=C:\\Game\\game.exe action=PROXY protocol=BOTH proxy=SOCKS5://127.0.0.1:7891"));
    CHECK(!strstr(summary, "action=BLOCK"));
    CHECK(guard_id == 0 && installed[rule_ids[2]].action == RULE_ACTION_PROXY);
    CHECK(strstr(summary, "local/link-local/multicast/broadcast destinations -> DIRECT (hosts=127.*.*.*;"));
    CHECK(!strstr(summary, "* -> DIRECT"));

    // A real committed BLOCK must stay in the info summary after guard removal.
    CHECK(replace_and_capture(blocked_rule, TRUE, &replaced) && replaced);
    summary = strstr(captured, "Active routing rules committed:");
    CHECK(summary && strstr(summary, "action=BLOCK protocol=TCP"));
    CHECK(!strstr(summary, "proxy=SOCKS5"));
    CHECK(installed[rule_ids[2]].action == RULE_ACTION_BLOCK);

    CHECK(replace_and_capture(with_disabled, TRUE, &replaced) && replaced);
    CHECK(strstr(captured, "applications=1 exclusions=2"));
    CHECK(strstr(captured, "Active application rule: priority=1"));
    CHECK(strstr(captured, "action=DIRECT protocol=UDP"));
    CHECK(strstr(captured, "level=debug Inactive application rule: priority=2 process=disabled.exe"));
    CHECK(!strstr(captured, "Active application rule: priority=2"));
    CHECK(!installed[rule_ids[3]].enabled);

    // Opting out silences the entire rebuild, not just the final snapshot.
    CHECK(replace_and_capture(proxy_rule, FALSE, &replaced) && replaced);
    CHECK(captured[0] == '\0' && fake_logger == NULL);

    fail_commit = TRUE;
    CHECK(replace_and_capture(proxy_rule, TRUE, &replaced) && !replaced);
    CHECK(!strstr(captured, "Active routing rules committed:") && guard_id != 0);
    fail_commit = FALSE;

    engine_running = FALSE;
    fail_start = TRUE;
    CHECK(replace_and_capture(proxy_rule, TRUE, &replaced) && !replaced);
    CHECK(!strstr(captured, "Active routing rules committed:") && guard_id != 0);
    clear_rules();
    ProxyBridge_DeleteRule(guard_id);
    puts("PASS: committed rule summaries, guard/start failure, disabled rules, and diagnostic opt-out");
    return 0;
}
