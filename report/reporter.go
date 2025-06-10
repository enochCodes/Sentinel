package report

import (
    "fmt"
    "io"
    "math/rand"
    "net/http"
    "net/url"
    "os"
    "time"
)

var userAgents = []string{
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 14_2 like Mac OS X)",
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
}

func ReportLoop(fullURL string, proxies []string, maxAttempts int, logPath string) {
    client := &http.Client{}
    logfile, _ := os.Create(logPath)
    defer logfile.Close()

    for i := 0; i < maxAttempts; i++ {
        proxy := proxies[rand.Intn(len(proxies))]
        proxyURL, _ := url.Parse(proxy)

        transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
        client.Transport = transport

        req, _ := http.NewRequest("POST", fullURL, nil)
        req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
        req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

        fmt.Printf("[*] Sending report %d using proxy: %s\n", i+1, proxy)
        resp, err := client.Do(req)
        if err != nil {
            fmt.Println("[x] Request failed:", err)
            logfile.WriteString(fmt.Sprintf("Attempt %d: FAILED\n", i+1))
            break
        }

        body, _ := io.ReadAll(resp.Body)
        fmt.Printf("[✓] Response code: %d | Body: %s\n", resp.StatusCode, string(body))
        logfile.WriteString(fmt.Sprintf("Attempt %d: %s\n", i+1, string(body)))
        resp.Body.Close()

        time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
    }

    fmt.Println("[✓] Reporting session completed.")
}
