// Package i18n localizes user-visible KokoroBox Service messages.
package i18n

import (
	"os"
	"strings"
	"sync"
)

// Locale identifies a supported display language.
type Locale string

const (
	English            Locale = "en"
	SimplifiedChinese  Locale = "zh-CN"
	TraditionalChinese Locale = "zh-TW"

	// Environment selects the process-wide fallback language. HTTP clients may
	// override it for an individual request with Accept-Language.
	Environment = "KOKOROBOX_LOCALE"
)

var (
	defaultLocale = Normalize(os.Getenv(Environment))
	localeMu      sync.RWMutex

	simplifiedCatalog   = buildSimplifiedCatalog(simplifiedSourceTerms)
	traditionalCatalog  = buildTraditionalCatalog(simplifiedSourceTerms)
	traditionalReplacer = strings.NewReplacer(traditionalCharacters...)
)

// Normalize accepts the documented values en, zh-CN, and zh-TW, plus
// compatible language tags used by operating systems and HTTP clients. Unknown
// values fall back to English so a newly installed service is usable without
// configuration.
func Normalize(value string) Locale {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ",")[0]))
	value = strings.ReplaceAll(value, "_", "-")
	switch {
	case value == "zh-cn", value == "zh-hans", strings.HasPrefix(value, "zh-hans-"):
		return SimplifiedChinese
	case value == "zh-tw", value == "zh-hant", strings.HasPrefix(value, "zh-hant-"):
		return TraditionalChinese
	default:
		return English
	}
}

// FromAcceptLanguage returns the first supported language in an HTTP header.
func FromAcceptLanguage(value string) Locale {
	for part := range strings.SplitSeq(value, ",") {
		tag := strings.TrimSpace(strings.Split(part, ";")[0])
		if tag == "" {
			continue
		}
		locale := Normalize(tag)
		if locale == SimplifiedChinese || locale == TraditionalChinese || strings.HasPrefix(strings.ToLower(tag), "en") {
			return locale
		}
	}
	return Default()
}

// Default returns the fallback language used for CLI output and logs.
func Default() Locale {
	localeMu.RLock()
	defer localeMu.RUnlock()
	return defaultLocale
}

// SetDefault changes the process-wide fallback language. It is intended for
// the command-line --locale flag before the service starts.
func SetDefault(value string) {
	localeMu.Lock()
	defer localeMu.Unlock()
	defaultLocale = Normalize(value)
}

// Text translates an English source message. English is the source language
// and fallback for missing translations.
func Text(locale Locale, message string) string {
	switch Normalize(string(locale)) {
	case SimplifiedChinese:
		if translated, ok := simplifiedCatalog[message]; ok {
			return translated
		}
	case TraditionalChinese:
		if translated, ok := traditionalCatalog[message]; ok {
			return translated
		}
	}
	return message
}

// DefaultText translates a message using the process-wide fallback language.
func DefaultText(message string) string {
	return Text(Default(), message)
}

func buildSimplifiedCatalog(sourceTerms map[string]string) map[string]string {
	catalog := make(map[string]string, len(sourceTerms))
	for simplified, english := range sourceTerms {
		catalog[english] = simplified
	}
	return catalog
}

func buildTraditionalCatalog(sourceTerms map[string]string) map[string]string {
	catalog := make(map[string]string, len(sourceTerms))
	for simplified, english := range sourceTerms {
		traditional, ok := traditionalTerms[simplified]
		if !ok {
			traditional = traditionalReplacer.Replace(simplified)
		}
		catalog[english] = traditional
	}
	return catalog
}

// simplifiedSourceTerms is the zh-CN catalog. Its values are English source
// messages; callers must pass those English values to Text.
var simplifiedSourceTerms = map[string]string{
	"输出语言：en、zh-CN 或 zh-TW":                                     "Output language: en, zh-CN, or zh-TW",
	"错误：必须通过 --public-key 参数提供公钥":                               "Error: a public key must be provided with --public-key",
	"错误：必须通过 --authorized-sid 或 --authorized-uid 绑定允许访问服务的用户身份": "Error: an authorized user identity must be bound with --authorized-sid or --authorized-uid",
	"查询服务状态失败；如果服务正在运行，请手动执行 'restart' 命令":                      "Failed to query service status; if the service is running, run 'restart' manually",
	"正在重启服务...": "Restarting service...",
	"重启服务失败；请手动执行 'kokorobox-service service restart' 命令": "Failed to restart service; run 'kokorobox-service service restart' manually",
	"服务重启中...":                 "Service is restarting...",
	"启动 KokoroBox 服务（测试用）":     "Start KokoroBox Service (for testing)",
	"安装 KokoroBox 服务":          "Install KokoroBox Service",
	"卸载 KokoroBox 服务":          "Uninstall KokoroBox Service",
	"启动 KokoroBox 服务":          "Start KokoroBox Service",
	"停止 KokoroBox 服务":          "Stop KokoroBox Service",
	"重启 KokoroBox 服务":          "Restart KokoroBox Service",
	"查看 KokoroBox 服务状态":        "Show KokoroBox Service status",
	"运行 KokoroBox 服务":          "Run KokoroBox Service",
	"管理 KokoroBox 服务":          "Manage KokoroBox Service",
	"初始化服务（传入公钥）":              "Initialize the service with a public key",
	"管理系统代理设置":                 "Manage system proxy settings",
	"设置系统代理":                   "Set system proxy",
	"设置 PAC 代理":                "Set PAC proxy",
	"取消代理设置":                   "Disable proxy settings",
	"查看当前代理设置":                 "Show current proxy settings",
	"仅对活跃的网络设备生效":              "Apply only to active network devices",
	"使用注册表设置":                  "Use registry settings",
	"指定网络设备":                   "Specify network device",
	"监听地址":                     "Listen address",
	"代理服务器地址":                  "Proxy server address",
	"绕过地址":                     "Bypass addresses",
	"PAC 地址":                   "PAC URL",
	"客户端公钥":                    "Client public key",
	"允许访问服务的 Windows SID":      "Windows SID allowed to access the service",
	"允许访问服务的 Unix UID":         "Unix UID allowed to access the service",
	"核心启动配置已更新":                "Core launch configuration updated",
	"核心启动成功":                   "Core started successfully",
	"核心停止成功":                   "Core stopped successfully",
	"核心重启成功":                   "Core restarted successfully",
	"核心未运行":                    "Core is not running",
	"核心正在启动":                   "Core is starting",
	"核心已启动":                    "Core started",
	"核心正在停止":                   "Core is stopping",
	"核心已停止":                    "Core stopped",
	"核心正在重启":                   "Core is restarting",
	"核心重启失败":                   "Failed to restart core",
	"核心启动失败":                   "Failed to start core",
	"核心进程已重新接管":                "Core process taken over again",
	"核心进程已退出":                  "Core process exited",
	"核心进程已在运行中":                "Core process is already running",
	"核心进程异常退出":                 "Core process exited unexpectedly",
	"核心进程已终止":                  "Core process terminated",
	"服务初始化成功，认证配置已更新":          "Service initialized; authentication configuration updated",
	"服务初始化成功，认证配置未变化":          "Service initialized; authentication configuration unchanged",
	"服务已在运行，配置未变化，无需重启":        "Service is already running; configuration is unchanged and no restart is needed",
	"服务未运行，配置将在下次启动时生效":        "Service is not running; configuration will apply on the next start",
	"服务已成功重启":                  "Service restarted successfully",
	"服务安装成功":                   "Service installed successfully",
	"服务卸载成功":                   "Service uninstalled successfully",
	"服务启动成功":                   "Service started successfully",
	"服务停止成功":                   "Service stopped successfully",
	"服务启动中...":                 "Service is starting...",
	"服务停止中...":                 "Service is stopping...",
	"服务已停止":                    "Service is stopped",
	"服务未运行":                    "Service is not running",
	"服务状态：运行中":                 "Service status: running",
	"服务状态：已停止":                 "Service status: stopped",
	"服务状态：未知":                  "Service status: unknown",
	"DNS 设置成功":                 "DNS settings updated successfully",
	"系统代理守护未运行":                "System proxy guard is not running",
	"系统代理守护已启动":                "System proxy guard started",
	"系统代理守护已停止":                "System proxy guard stopped",
	"系统代理守护启动失败，已停止":           "Failed to start system proxy guard; it has stopped",
	"系统代理守护检测到代理设置被修改":         "System proxy guard detected proxy settings were changed",
	"系统代理守护已恢复代理设置":            "System proxy guard restored proxy settings",
	"查询代理设置完成":                 "Queried proxy settings",
	"查询代理设置失败":                 "Failed to query proxy settings",
	"设置 PAC 完成":                "PAC settings updated",
	"设置 PAC 失败":                "Failed to update PAC settings",
	"设置代理完成":                   "Proxy settings updated",
	"设置代理失败":                   "Failed to update proxy settings",
	"设置代理失败：":                  "Failed to update proxy settings: ",
	"设置 PAC 代理失败：":             "Failed to update PAC proxy settings: ",
	"禁用代理完成":                   "Proxy disabled",
	"禁用代理失败":                   "Failed to disable proxy",
	"代理设置成功，耗时：":               "Proxy settings updated in: ",
	"PAC 代理设置成功，耗时：":           "PAC proxy settings updated in: ",
	"代理设置已取消，耗时：":              "Proxy settings disabled in: ",
	"取消代理设置失败：":                "Failed to disable proxy settings: ",
	"格式化 JSON 失败：":             "Failed to format JSON: ",
	"无效的请求体:":                  "Invalid request body:",
	"无效的请求体: %v":               "Invalid request body: %v",
	"无效的请求体：":                  "Invalid request body: ",
	"服务未初始化":                   "Service is not initialized",
	"请求方未授权:":                  "Requestor is not authorized:",
	"请求方未授权: %v":               "Requestor is not authorized: %v",
	"请求方未授权：":                  "Requestor is not authorized: ",
	"仅支持 Auth V2/V3":           "Only Auth V2/V3 is supported",
	"缺少认证信息":                   "Missing authentication information",
	"无效的时间戳格式":                 "Invalid timestamp format",
	"请求已过期或时间戳无效":              "Request expired or timestamp is invalid",
	"请求体摘要不匹配":                 "Request body digest does not match",
	"请求已重放":                    "Request has already been replayed",
	"websocket 仅支持 GET":        "WebSocket only supports GET",
	"缺少 websocket upgrade 请求头": "Missing WebSocket upgrade request header",
	"不支持的 websocket 版本":        "Unsupported WebSocket version",
	"缺少 Sec-WebSocket-Key":     "Missing Sec-WebSocket-Key",
	"无效的 Sec-WebSocket-Key":    "Invalid Sec-WebSocket-Key",
	"websocket 不支持分片帧":         "WebSocket does not support fragmented frames",
	"websocket 客户端帧必须 mask":    "WebSocket client frames must be masked",
	"websocket payload 过大":     "WebSocket payload is too large",
	"创建服务失败":                   "Failed to create service",
	"安装服务失败":                   "Failed to install service",
	"卸载服务失败":                   "Failed to uninstall service",
	"启动服务失败":                   "Failed to start service",
	"停止服务失败":                   "Failed to stop service",
	"重启服务失败":                   "Failed to restart service",
	"查询服务状态失败":                 "Failed to query service status",
	"准备服务数据目录失败":               "Failed to prepare service data directory",
	"设置公钥失败":                   "Failed to set public key",
	"设置授权 SID 失败":              "Failed to set authorized SID",
	"设置授权 UID 失败":              "Failed to set authorized UID",
	"清理应用分流防火墙失败":              "Failed to remove application-routing firewall rules",
	"必须通过 --public-key 参数提供公钥": "A public key must be provided with --public-key",
	"必须通过 --authorized-sid 或 --authorized-uid 绑定允许访问服务的用户身份": "An authorized user identity must be bound with --authorized-sid or --authorized-uid",
	"读取请求体失败：":        "Failed to read request body: ",
	"规范化请求参数失败：":      "Failed to normalize request parameters: ",
	"不支持的认证版本:":       "Unsupported authentication version:",
	"不支持的认证版本：":       "Unsupported authentication version: ",
	"公钥不能为空":          "Public key cannot be empty",
	"公钥 base64 解码失败：": "Failed to decode public key Base64: ",
	"解析公钥失败：":         "Failed to parse public key: ",
	"公钥不是 Ed25519 类型": "Public key is not an Ed25519 key",
	"密钥 ID 不能为空":      "Key ID cannot be empty",
	"密钥 ID 过长":        "Key ID is too long",
	"密钥 ID 格式无效":      "Invalid key ID format",
	"密钥 ID 与公钥不匹配":    "Key ID does not match public key",
	"密钥 ID 未注册":       "Key ID is not registered",
	"签名解码失败：":         "Failed to decode signature: ",
	"签名验证失败":          "Signature verification failed",
	"授权主体为空":          "Authorized principal is empty",
	"UID 不能为空":        "UID cannot be empty",
	"SID 不能为空":        "SID cannot be empty",
	"SID 格式无效":        "Invalid SID format",
	"授权主体文件不存在（未绑定请求方身份）":   "Authorized principal file does not exist (requestor identity is not bound)",
	"当前请求未携带可识别的本地身份信息":     "Request does not carry a recognizable local identity",
	"请求方身份类型不匹配":            "Requestor identity type does not match",
	"请求方身份不匹配":              "Requestor identity does not match",
	"读取 service 可执行文件路径失败：": "Failed to read service executable path: ",
	"创建服务运行目录失败：":           "Failed to create service runtime directory: ",
	"创建服务运行副本失败：":           "Failed to create service runtime copy: ",
	"检查服务运行副本失败：":           "Failed to check service runtime copy: ",
	"打开服务二进制失败：":            "Failed to open service binary: ",
	"复制服务二进制失败：":            "Failed to copy service binary: ",
	"关闭服务二进制失败：":            "Failed to close service binary: ",
	"发布服务运行副本失败：":           "Failed to publish service runtime copy: ",
	"服务路径指向目录而非可执行文件:":      "Service path points to a directory, not an executable:",
	"核心沙盒需要 root 权限":        "Core sandbox requires root privileges",
	"需要 root 权限":            "Root privileges are required",
	"核心启动命令为空":              "Core launch command is empty",
	"核心控制器未初始化":             "Core controller is not initialized",
	"进程未运行":                 "Process is not running",
	"不支持的操作系统:":             "Unsupported operating system:",
	"不支持的进程优先级:":            "Unsupported process priority:",
	"当前连接不支持 websocket":     "Current connection does not support WebSocket",
	"转发核心控制器请求失败：%w":        "Failed to forward core controller request: %w",
	"HTTP 请求完成":             "HTTP request completed",
	"编码 HTTP JSON 响应失败：":    "Failed to encode HTTP JSON response: ",
}

// zh-TW uses a few terms that differ from a mechanical Simplified-to-
// Traditional conversion, such as 網路 rather than 網絡 and 設定 rather than
// 設置. Keep these user-facing phrases in a small locale-specific catalog.
var traditionalTerms = map[string]string{
	"启动 KokoroBox 服务（测试用）":  "啟動 KokoroBox 服務（測試用）",
	"安装 KokoroBox 服务":       "安裝 KokoroBox 服務",
	"卸载 KokoroBox 服务":       "解除安裝 KokoroBox 服務",
	"启动 KokoroBox 服务":       "啟動 KokoroBox 服務",
	"停止 KokoroBox 服务":       "停止 KokoroBox 服務",
	"重启 KokoroBox 服务":       "重新啟動 KokoroBox 服務",
	"查看 KokoroBox 服务状态":     "查看 KokoroBox 服務狀態",
	"运行 KokoroBox 服务":       "執行 KokoroBox 服務",
	"管理 KokoroBox 服务":       "管理 KokoroBox 服務",
	"初始化服务（传入公钥）":           "初始化服務（傳入公鑰）",
	"管理系统代理设置":              "管理系統代理設定",
	"设置系统代理":                "設定系統代理",
	"设置 PAC 代理":             "設定 PAC 代理",
	"取消代理设置":                "停用代理設定",
	"查看当前代理设置":              "查看目前代理設定",
	"仅对活跃的网络设备生效":           "僅對使用中的網路介面生效",
	"使用注册表设置":               "使用登錄檔設定",
	"指定网络设备":                "指定網路介面",
	"监听地址":                  "監聽位址",
	"代理服务器地址":               "代理伺服器位址",
	"绕过地址":                  "略過位址",
	"PAC 地址":                "PAC 網址",
	"客户端公钥":                 "用戶端公鑰",
	"允许访问服务的 Windows SID":   "允許存取服務的 Windows SID",
	"允许访问服务的 Unix UID":      "允許存取服務的 Unix UID",
	"核心启动配置已更新":             "核心啟動設定已更新",
	"核心启动成功":                "核心啟動成功",
	"核心停止成功":                "核心已停止",
	"核心重启成功":                "核心重新啟動成功",
	"核心未运行":                 "核心未執行",
	"核心正在启动":                "核心正在啟動",
	"核心已启动":                 "核心已啟動",
	"核心正在停止":                "核心正在停止",
	"核心已停止":                 "核心已停止",
	"核心正在重启":                "核心正在重新啟動",
	"核心重启失败":                "核心重新啟動失敗",
	"核心启动失败":                "核心啟動失敗",
	"服务安装成功":                "服務安裝成功",
	"服务卸载成功":                "服務解除安裝成功",
	"服务启动成功":                "服務啟動成功",
	"服务停止成功":                "服務停止成功",
	"服务启动中...":              "服務正在啟動...",
	"服务停止中...":              "服務正在停止...",
	"服务已停止":                 "服務已停止",
	"服务未运行":                 "服務未執行",
	"服务状态：运行中":              "服務狀態：執行中",
	"服务状态：已停止":              "服務狀態：已停止",
	"服务状态：未知":               "服務狀態：未知",
	"DNS 设置成功":              "DNS 設定成功",
	"无效的请求体":                "無效的請求主體",
	"服务未初始化":                "服務尚未初始化",
	"请求方未授权":                "請求端未獲授權",
	"仅支持 Auth V2/V3":        "僅支援 Auth V2/V3",
	"缺少认证信息":                "缺少驗證資訊",
	"无效的时间戳格式":              "無效的時間戳記格式",
	"请求已过期或时间戳无效":           "請求已過期或時間戳記無效",
	"请求体摘要不匹配":              "請求主體摘要不符",
	"请求已重放":                 "請求已重放",
	"设置代理失败：":               "設定代理失敗：",
	"代理设置成功，耗时：":            "代理設定成功，耗時：",
	"PAC 代理设置成功，耗时：":        "PAC 代理設定成功，耗時：",
	"代理设置已取消，耗时：":           "已停用代理設定，耗時：",
	"取消代理设置失败：":             "停用代理設定失敗：",
	"查询代理设置失败：":             "查詢代理設定失敗：",
	"格式化 JSON 失败：":          "格式化 JSON 失敗：",
	"输出语言：en 或 zh-TW":       "輸出語言：en 或 zh-TW",
	"输出语言：en、zh-CN 或 zh-TW": "輸出語言：en、zh-CN 或 zh-TW",
}

// This character table converts source messages (which historically used
// Simplified Chinese) to Traditional Chinese. Non-Chinese data, such as paths
// and operating-system errors, is deliberately left untouched.
var traditionalCharacters = []string{
	"务", "務", "启", "啟", "动", "動", "状", "狀", "态", "態", "运", "運", "行", "行", "已", "已", "停", "停", "止", "止", "装", "裝", "卸", "卸", "载", "載", "查", "查", "询", "詢", "创", "創", "建", "建", "设", "設", "置", "置", "错", "錯", "误", "誤", "须", "須", "过", "過", "户", "戶", "许", "許", "访", "訪", "问", "問", "认", "認", "证", "證", "变", "變", "更", "更", "无", "無", "应", "應", "该", "該", "执", "執", "数", "數", "据", "據", "录", "錄", "钥", "鑰", "类", "類", "别", "別", "绑", "綁", "请", "請", "将", "將", "经", "經", "时", "時", "间", "間", "复", "復", "项", "項", "处", "處", "线", "線", "统", "統", "监", "監", "护", "護", "检", "檢", "测", "測", "码", "碼", "体", "體", "签", "簽", "验", "驗", "读", "讀", "写", "寫", "关", "關", "闭", "閉", "开", "開", "发", "發", "布", "佈", "权", "權", "优", "優", "级", "級", "当", "當", "连", "連", "仅", "僅", "支", "支", "获", "獲", "扩", "擴", "网", "網", "络", "絡", "协", "協", "议", "議", "响", "響", "内", "內", "进", "進", "终", "終", "异", "異", "转", "轉", "换", "換", "试", "試", "达", "達", "输", "輸", "虚", "虛", "拟", "擬", "挂", "掛", "离", "離", "径", "徑", "软", "軟", "听", "聽", "记", "記", "调", "調", "显", "顯", "缓", "緩", "冲", "衝", "断", "斷", "页", "頁", "标", "標", "准", "準", "释", "釋", "为", "為", "与", "與", "号", "號", "组", "組", "员", "員", "帐", "帳", "墙", "牆", "对", "對", "跃", "躍", "备", "備", "册", "冊", "注", "註", "务", "務",
}
