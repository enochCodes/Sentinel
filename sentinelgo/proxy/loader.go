package proxy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// ProxyInfo holds detailed information about a proxy.
type ProxyInfo struct {
	URL            *url.URL
	OriginalString string
	Source         string
	Region         string
	HealthStatus   string
	LastChecked    time.Time
	Latency        time.Duration
}

// parseProxyString attempts to parse a raw proxy string into a *url.URL.
// It handles common formats like ip:port, user:pass@ip:port, or full URLs.
func parseProxyString(proxyStr string, defaultScheme string) (*url.URL, error) {
	if !strings.Contains(proxyStr, "://") {
		proxyStr = defaultScheme + "://" + proxyStr
	}
	parsedURL, err := url.Parse(proxyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL %s: %w", proxyStr, err)
	}
	// Ensure Userinfo is correctly parsed if present in host part, e.g. user:pass@host:port
	if parsedURL.User == nil && strings.Contains(parsedURL.Host, "@") {
		parts := strings.SplitN(parsedURL.Host, "@", 2)
		if len(parts) == 2 {
			userInfoStr := parts[0]
			hostStr := parts[1]

			userinfoParts := strings.SplitN(userInfoStr, ":", 2)
			username := userinfoParts[0]
			password := ""
			if len(userinfoParts) > 1 {
				password = userinfoParts[1]
			}
			parsedURL.User = url.UserPassword(username, password)
			parsedURL.Host = hostStr
		} else {
            return nil, fmt.Errorf("malformed user info in proxy string: %s", proxyStr)
        }
	}


	return parsedURL, nil
}

// LoadProxies loads proxy information from various sources (CSV, JSON files, or API endpoints).
func LoadProxies(sourcePathOrURL string) ([]*ProxyInfo, error) {
	var proxies []*ProxyInfo

	if strings.HasPrefix(sourcePathOrURL, "http://") || strings.HasPrefix(sourcePathOrURL, "https://") {
		fmt.Printf("API proxy loading not yet implemented for %s\n", sourcePathOrURL)
		// In a real implementation, you might return an error or an empty slice.
		// For this subtask, returning an empty slice and no error as per placeholder.
		return proxies, nil
	} else if strings.HasSuffix(sourcePathOrURL, ".json") {
		// Load from JSON file
		file, err := os.ReadFile(sourcePathOrURL)
		if err != nil {
			return nil, fmt.Errorf("failed to read JSON proxy file %s: %w", sourcePathOrURL, err)
		}

		var rawProxies []struct {
			Proxy  string `json:"proxy"`
			Region string `json:"region"`
		}
		err = json.Unmarshal(file, &rawProxies)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON proxies from %s: %w", sourcePathOrURL, err)
		}

		for _, rawP := range rawProxies {
			if rawP.Proxy == "" {
				// Skip entries without a proxy URL
				fmt.Printf("Skipping empty proxy entry in %s\n", sourcePathOrURL)
				continue
			}
			parsedURL, err := parseProxyString(rawP.Proxy, "http") // Assuming http if not specified
			if err != nil {
				fmt.Printf("Skipping unparseable proxy '%s' from %s: %v\n", rawP.Proxy, sourcePathOrURL, err)
				continue
			}
			proxies = append(proxies, &ProxyInfo{
				URL:            parsedURL,
				OriginalString: rawP.Proxy,
				Source:         sourcePathOrURL,
				Region:         rawP.Region,
				HealthStatus:   "unknown",
			})
		}
	} else if strings.HasSuffix(sourcePathOrURL, ".csv") {
		// Load from CSV file
		file, err := os.Open(sourcePathOrURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open CSV proxy file %s: %w", sourcePathOrURL, err)
		}
		defer file.Close()

		reader := csv.NewReader(file)
		// Allow variable number of fields to handle optional region
		reader.FieldsPerRecord = -1
		lines, err := reader.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV proxies from %s: %w", sourcePathOrURL, err)
		}

		for i, line := range lines {
			if i == 0 && (strings.ToLower(line[0]) == "ip" || strings.ToLower(line[0]) == "host") { // Skip header
				continue
			}
			if len(line) < 1 { // Skip empty lines
				continue
			}

			var proxyStr, region string

			// Handle format ip,port,user,pass or ip:port:user:pass
			if len(line) == 1 && strings.Contains(line[0], ":") { // ip:port:user:pass format
				parts := strings.Split(line[0], ":")
				if len(parts) < 2 {
					fmt.Printf("Skipping malformed CSV line (format ip:port:user:pass) #%d in %s: %v\n", i+1, sourcePathOrURL, line)
					continue
				}
				ipPort := strings.Join(parts[0:2], ":")
				if len(parts) >= 4 { // user:pass present
                    userInfo := strings.Join(parts[2:4], ":")
                    proxyStr = fmt.Sprintf("http://%s@%s", userInfo, ipPort)
					if len(parts) >= 5 {
						region = parts[4]
					}
                } else { // No user:pass
                    proxyStr = fmt.Sprintf("http://%s", ipPort)
					if len(parts) >= 3 {
						region = parts[2] // if region is 3rd element after ip:port
					}
                }

			} else if len(line) >= 4 { // ip,port,user,pass format
				ip, port, user, pass := line[0], line[1], line[2], line[3]
				proxyStr = fmt.Sprintf("http://%s:%s@%s:%s", user, pass, ip, port)
				if len(line) >= 5 {
					region = line[4]
				}
			} else if len(line) >= 2 { // ip,port format (no auth)
				ip, port := line[0], line[1]
				proxyStr = fmt.Sprintf("http://%s:%s", ip, port)
				if len(line) >=3 {
					region = line[2]
				}
			} else {
				fmt.Printf("Skipping malformed CSV line #%d in %s: %v\n", i+1, sourcePathOrURL, line)
				continue
			}

			parsedURL, err := parseProxyString(proxyStr, "http")
			if err != nil {
				fmt.Printf("Skipping unparseable proxy '%s' from CSV line #%d in %s: %v\n", proxyStr, i+1, sourcePathOrURL, err)
				continue
			}

			proxies = append(proxies, &ProxyInfo{
				URL:            parsedURL,
				OriginalString: strings.Join(line, ","), // Store original line
				Source:         sourcePathOrURL,
				Region:         region,
				HealthStatus:   "unknown",
			})
		}
	} else {
		return nil, fmt.Errorf("unsupported proxy source format: %s (must be .csv, .json, or http(s) URL)", sourcePathOrURL)
	}

	return proxies, nil
}
