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

// ProxyInfo holds detailed information about a single proxy server.
// This includes its parsed URL, original string representation, source,
// geographical region, health status, and performance metrics.
type ProxyInfo struct {
	// URL is the parsed `url.URL` object for the proxy.
	// This is used by the HTTP client to route requests.
	URL *url.URL

	// OriginalString is the raw string from which the proxy was loaded (e.g., "ip:port:user:pass").
	OriginalString string

	// Source indicates where this proxy was loaded from (e.g., filename, API endpoint).
	Source string

	// Region is an optional geographical region code for the proxy (e.g., "US", "EU").
	Region string

	// HealthStatus indicates the current known health of the proxy.
	// Common values: "unknown", "healthy", "unhealthy", "slow".
	HealthStatus string

	// LastChecked is the timestamp of the last health check performed on this proxy.
	LastChecked time.Time

	// Latency is the duration of the last successful health check request.
	Latency time.Duration
}

// parseProxyString attempts to parse a raw proxy string into a `*url.URL` object.
// It handles common proxy formats like `ip:port`, `user:pass@ip:port`, or full URLs
// (e.g., `http://user:pass@ip:port`). If `proxyStr` does not include a scheme (e.g., "http://"),
// the `defaultScheme` is prepended.
// It also includes logic to correctly parse user information if `url.Parse` initially
// includes it as part of the host.
func parseProxyString(proxyStr string, defaultScheme string) (*url.URL, error) {
	if proxyStr == "" {
		return nil, fmt.Errorf("proxy string cannot be empty")
	}
	if !strings.Contains(proxyStr, "://") {
		proxyStr = defaultScheme + "://" + proxyStr
	}
	parsedURL, err := url.Parse(proxyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL '%s': %w", proxyStr, err)
	}

	// Ensure Userinfo is correctly parsed if present in host part but not in parsedURL.User.
	// This handles cases like "user:pass@host:port" without a scheme, where url.Parse might
	// put "user:pass@" into parsedURL.Host initially.
	if parsedURL.User == nil && strings.Contains(parsedURL.Host, "@") {
		parts := strings.SplitN(parsedURL.Host, "@", 2)
		if len(parts) == 2 {
			userInfoStr := parts[0]
			hostStr := parts[1]

			var username, password string
			if strings.Contains(userInfoStr, ":") {
				userinfoParts := strings.SplitN(userInfoStr, ":", 2)
				username = userinfoParts[0]
				password = userinfoParts[1]
			} else {
				username = userInfoStr
			}
			parsedURL.User = url.UserPassword(username, password)
			parsedURL.Host = hostStr
		} else {
			// This case should ideally not be reached if proxyStr contains "@" in host
			// and SplitN with 2 parts doesn't yield two elements.
			return nil, fmt.Errorf("malformed user info in proxy string host part: %s", proxyStr)
		}
	}
	if parsedURL.Host == "" { // After parsing, host should not be empty
		return nil, fmt.Errorf("parsed proxy URL '%s' has empty host", parsedURL.String())
	}

	return parsedURL, nil
}

// LoadProxies loads proxy information from various sources specified by `sourcePathOrURL`.
// It supports loading from:
// - CSV files (ending in ".csv"): Expects lines in `ip,port,user,pass[,region]` or `ip:port:user:pass[:region]` format.
// - JSON files (ending in ".json"): Expects a JSON array of objects, each with a "proxy" string field
//   (e.g., "http://user:pass@host:port") and an optional "region" field.
// - HTTP(S) URLs (starting with "http://" or "https://"): Currently a placeholder; it logs a message
//   that API proxy loading is not yet implemented and returns an empty list.
//
// For file-based sources, it skips header lines (if "ip" or "host" is the first field in CSV)
// and malformed or empty entries, logging these skips to standard output.
// It returns a slice of `*ProxyInfo` structs or an error if critical issues occur (e.g., file not found,
// unmarshal errors for entire file, unsupported format).
func LoadProxies(sourcePathOrURL string) ([]*ProxyInfo, error) {
	var proxies []*ProxyInfo

	if strings.HasPrefix(sourcePathOrURL, "http://") || strings.HasPrefix(sourcePathOrURL, "https://") {
		// Placeholder for API loading
		fmt.Printf("Notice: API proxy loading not yet implemented for URL: %s\n", sourcePathOrURL)
		return proxies, nil // Return empty slice, no error, as per current design
	} else if strings.HasSuffix(strings.ToLower(sourcePathOrURL), ".json") {
		// Load from JSON file
		fileData, err := os.ReadFile(sourcePathOrURL)
		if err != nil {
			return nil, fmt.Errorf("failed to read JSON proxy file '%s': %w", sourcePathOrURL, err)
		}

		// Define a struct for unmarshaling JSON entries
		var rawJSONProxies []struct {
			Proxy  string `json:"proxy"`  // Expected format: "scheme://user:pass@host:port" or "host:port" etc.
			Region string `json:"region"` // Optional region string
		}
		err = json.Unmarshal(fileData, &rawJSONProxies)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON proxies from '%s': %w", sourcePathOrURL, err)
		}

		for _, rawP := range rawJSONProxies {
			if rawP.Proxy == "" {
				fmt.Printf("Skipping empty proxy entry in JSON file '%s'\n", sourcePathOrURL)
				continue
			}
			// Assuming "http" as default scheme if not specified in the proxy string itself.
			parsedURL, err := parseProxyString(rawP.Proxy, "http")
			if err != nil {
				fmt.Printf("Skipping unparseable proxy string '%s' from JSON file '%s': %v\n", rawP.Proxy, sourcePathOrURL, err)
				continue
			}
			proxies = append(proxies, &ProxyInfo{
				URL:            parsedURL,
				OriginalString: rawP.Proxy,
				Source:         sourcePathOrURL,
				Region:         rawP.Region,
				HealthStatus:   "unknown", // Default health status
			})
		}
	} else if strings.HasSuffix(strings.ToLower(sourcePathOrURL), ".csv") {
		// Load from CSV file
		file, err := os.Open(sourcePathOrURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open CSV proxy file '%s': %w", sourcePathOrURL, err)
		}
		defer file.Close()

		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1 // Allow variable number of fields per line
		reader.TrimLeadingSpace = true

		lines, err := reader.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV proxies from '%s': %w", sourcePathOrURL, err)
		}

		for i, line := range lines {
			if len(line) == 0 { // Skip completely empty lines
				continue
			}
			// Skip header line if it looks like one (case-insensitive check for "ip" or "host" in first field)
			if i == 0 && (strings.ToLower(line[0]) == "ip" || strings.ToLower(line[0]) == "host" || strings.ToLower(line[0]) == "proxy") {
				continue
			}
			if len(line) == 1 && line[0] == "" { // Skip lines with only a single empty field
				continue
			}

			var proxyStr, region string
			originalLineStr := strings.Join(line, ",") // For ProxyInfo.OriginalString

			// Try to parse based on expected CSV structures
			if len(line) == 1 && strings.Contains(line[0], ":") { // Format: "ip:port:user:pass:region" or similar, all in one field
				parts := strings.Split(line[0], ":")
				if len(parts) < 2 { // Must have at least ip:port
					fmt.Printf("Skipping malformed colon-delimited CSV line #%d in '%s': %v\n", i+1, sourcePathOrURL, line)
					continue
				}

				hostPort := parts[0] + ":" + parts[1]
				userInfo := ""

				if len(parts) >= 4 { // user:pass present
					userInfo = parts[2] + ":" + parts[3]
					proxyStr = "http://" + userInfo + "@" + hostPort // Default scheme http
					if len(parts) >= 5 { region = parts[4] }
				} else if len(parts) == 3 { // ip:port:region or ip:port:user (assume region if not looking like user@)
					// This case is ambiguous. If parts[2] is purely alpha, could be user. If numeric or geo-code like, region.
					// For simplicity, assume if 3 parts, parts[2] is region. For user only, use user@ip:port.
					// A stricter format or more fields would be better.
					// Assuming ip:port:region for this case.
					proxyStr = "http://" + hostPort
					region = parts[2]
				} else { // Only ip:port
					proxyStr = "http://" + hostPort
				}
			} else if len(line) >= 2 { // Format: ip,port[,user,pass[,region]]
				host, port := line[0], line[1]
				userInfo := ""
				if len(line) >= 4 && line[2] != "" && line[3] != "" { // user,pass present
					userInfo = line[2] + ":" + line[3]
					proxyStr = "http://" + userInfo + "@" + host + ":" + port
				} else {
					proxyStr = "http://" + host + ":" + port
				}
				if len(line) >= 5 { region = line[4] }
				 else if len(line) == 3 && userInfo == "" { region = line[2] } // ip,port,region case
			} else {
				fmt.Printf("Skipping malformed comma-delimited CSV line #%d in '%s': %v\n", i+1, sourcePathOrURL, line)
				continue
			}

			parsedURL, err := parseProxyString(proxyStr, "http") // parseProxyString adds scheme if missing
			if err != nil {
				fmt.Printf("Skipping unparseable proxy '%s' from CSV line #%d in '%s': %v\n", proxyStr, i+1, sourcePathOrURL, err)
				continue
			}

			proxies = append(proxies, &ProxyInfo{
				URL:            parsedURL,
				OriginalString: originalLineStr,
				Source:         sourcePathOrURL,
				Region:         region,
				HealthStatus:   "unknown",
			})
		}
	} else {
		return nil, fmt.Errorf("unsupported proxy source format for '%s' (must end with .csv, .json, or be an http(s) URL)", sourcePathOrURL)
	}

	return proxies, nil
}
