package proxy

import (
    "encoding/csv"
    "fmt"
    //"net/url"
    "os"
)

func LoadProxies(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    reader := csv.NewReader(file)
    lines, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }

    var proxies []string
    for _, line := range lines {
        ip, port, user, pass := line[0], line[1], line[2], line[3]
        proxyURL := fmt.Sprintf("http://%s:%s@%s:%s", user, pass, ip, port)
        proxies = append(proxies, proxyURL)
    }
    return proxies, nil
}
