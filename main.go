package main

import (
    "bufio"
    "fmt"
    "os"
    "strconv"
    "strings"
    "time"

    "sentinel_go/proxy"
    "sentinel_go/report"
)

func main() {
    reader := bufio.NewReader(os.Stdin)

    fmt.Println("──────────────────────────────────────────────")
    fmt.Println("Sentinel CLI – TikTok Report Flooder (Go)")
    fmt.Println("──────────────────────────────────────────────")

    fmt.Print("Enter full TikTok report URL: ")
    fullURL, _ := reader.ReadString('\n')
    fullURL = strings.TrimSpace(fullURL)

    fmt.Print("Enter number of reports: ")
    maxStr, _ := reader.ReadString('\n')
    maxStr = strings.TrimSpace(maxStr)
    maxAttempts, _ := strconv.Atoi(maxStr)

    proxies, err := proxy.LoadProxies("config/proxies.csv")
    if err != nil {
        fmt.Println("[x] Failed to load proxies:", err)
        return
    }

    timestamp := time.Now().Format("20060102_150405")
    logPath := fmt.Sprintf("logs/report_log_%s.txt", timestamp)

    report.ReportLoop(fullURL, proxies, maxAttempts, logPath)
}
