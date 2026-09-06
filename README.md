# sparkle-service

這是 [xishang0128/sparkle-service](https://github.com/xishang0128/sparkle-service) 的 fork，供 KokoroBox 使用。

本服務以 Go 編寫，負責代理核心程序、系統代理及 DNS 設定，並透過 Unix Socket（Linux/macOS）或具名管道（Windows）提供本機 HTTP API。

## Fork 新增內容

- 在 Windows x64 上啟動並監管 KokoroBox Process Router。
- Mihomo 無法使用時，將已啟用的 Proxy 規則切換為 Block，避免意外直連。
- 驗證並持久化 Process Router 規則，支援狀態查詢、停止及清理 API。
- 透過客戶端租約回收異常離線後殘留的 Router 與 WinDivert 資源。

Process Router 原生元件須放在 `sparkle-service.exe` 旁的 `process-router` 目錄。這項整合目前僅支援 Windows 10/11 x64。

## 建置與測試

需要 Go 1.26 或更新版本。

```bash
go build -o sparkle-service .
go test ./...
```

安裝為系統服務（Windows 需使用系統管理員權限；Linux/macOS 需使用 root 權限）：

```bash
sparkle-service service install
```

## 文件

- [上游專案](https://github.com/xishang0128/sparkle-service)
- [原始完整 README](README.original.md)

## 授權

授權條款請見 [LICENSE](LICENSE)。
