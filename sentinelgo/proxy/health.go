package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// defaultHealthCheckURL is the endpoint used for default proxy health checks.
// It should be a reliable, fast, and lightweight endpoint. httpbin.org/get reflects the request's origin.
const defaultHealthCheckURL = "http://httpbin.org/get"

// CheckProxyHealth attempts to make a lightweight HTTP GET request via the given proxy
// to a specified health check URL (or a default one).
// It updates the proxy's `HealthStatus`, `Latency`, and `LastChecked` fields based on the outcome.
//
// Parameters:
//   - proxy: A pointer to the ProxyInfo struct for the proxy to be checked. This struct will be updated.
//   - timeout: The maximum duration to wait for the health check request to complete.
//   - healthCheckURL (optional): A variadic string. If provided, the first non-empty string
//     is used as the health check URL. Otherwise, `defaultHealthCheckURL` is used.
//
// Returns:
//   - `nil` if the proxy is considered healthy (e.g., HTTP 200 OK response).
//   - An error if the proxy is unhealthy (e.g., request error, timeout, non-200 status code),
//     or if the proxy's URL is nil. The error message provides context about the failure.
//
// The `proxy.HealthStatus` is set to "healthy" or "unhealthy".
// `proxy.LastChecked` is always updated to the current time.
// `proxy.Latency` records the duration of the health check request.
func CheckProxyHealth(proxy *ProxyInfo, timeout time.Duration, healthCheckURL ...string) error {
	checkURL := defaultHealthCheckURL
	if len(healthCheckURL) > 0 && healthCheckURL[0] != "" {
		checkURL = healthCheckURL[0]
	}

	// Ensure proxy and its URL are valid before proceeding.
	if proxy == nil {
		return fmt.Errorf("cannot check health of a nil ProxyInfo")
	}
	if proxy.URL == nil {
		proxy.HealthStatus = "unhealthy" // Mark as unhealthy if URL is nil
		proxy.LastChecked = time.Now()
		return fmt.Errorf("proxy '%s' (source: %s) has a nil URL", proxy.OriginalString, proxy.Source)
	}

	// Create an HTTP client configured to use the proxy and the specified timeout.
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxy.URL)},
		Timeout:   timeout,
	}

	startTime := time.Now()
	// Create a new request with context to allow for cancellation if needed, although client.Timeout is primary.
	req, err := http.NewRequestWithContext(context.Background(), "GET", checkURL, nil)
	if err != nil {
		proxy.HealthStatus = "unhealthy"
		proxy.LastChecked = time.Now()
		return fmt.Errorf("failed to create health check request for proxy '%s' to URL '%s': %w", proxy.OriginalString, checkURL, err)
	}

	// Perform the HTTP GET request.
	resp, err := client.Do(req)
	proxy.LastChecked = time.Now()      // Update last checked time regardless of outcome.
	proxy.Latency = time.Since(startTime) // Record latency.

	if err != nil {
		proxy.HealthStatus = "unhealthy"
		return fmt.Errorf("health check for proxy '%s' to URL '%s' failed: %w", proxy.OriginalString, checkURL, err)
	}
	defer resp.Body.Close() // Ensure the response body is closed.

	// Check if the status code indicates a healthy proxy.
	if resp.StatusCode == http.StatusOK {
		proxy.HealthStatus = "healthy"
	} else {
		proxy.HealthStatus = "unhealthy"
		return fmt.Errorf("health check for proxy '%s' to URL '%s' returned non-200 status: %d %s", proxy.OriginalString, checkURL, resp.StatusCode, resp.Status)
	}

	return nil
}

// BatchCheckProxies concurrently checks the health of a list of proxies.
// It uses a specified number of goroutines (`concurrency`) to perform checks in parallel.
//
// Parameters:
//   - proxies: A slice of `*ProxyInfo` structs to be checked. Each struct is updated by `CheckProxyHealth`.
//   - checkTimeout: The timeout duration for each individual proxy health check.
//   - concurrency: The maximum number of concurrent health check goroutines. If less than 1, it defaults to 1.
//   - healthCheckURL (optional): The URL(s) to use for health checks, passed to `CheckProxyHealth`.
//
// This function logs the outcome of each health check (success or failure with error details)
// to standard output using `fmt.Printf`. It does not return aggregated results or errors directly,
// relying on the updates to the `ProxyInfo` structs and the console logs for feedback.
func BatchCheckProxies(proxies []*ProxyInfo, checkTimeout time.Duration, concurrency int, healthCheckURL ...string) {
	if concurrency <= 0 {
		concurrency = 1 // Ensure at least one worker goroutine.
	}

	var wg sync.WaitGroup
	// Semaphore to limit the number of concurrent goroutines.
	semaphore := make(chan struct{}, concurrency)

	for _, p := range proxies {
		if p == nil { // Skip nil ProxyInfo entries
			fmt.Printf("Skipping health check for a nil ProxyInfo entry.\n")
			continue
		}
		wg.Add(1)
		semaphore <- struct{}{} // Acquire a slot in the semaphore.

		go func(proxyToCheck *ProxyInfo) {
			defer wg.Done()                // Signal completion for this goroutine.
			defer func() { <-semaphore }() // Release the slot in the semaphore.

			err := CheckProxyHealth(proxyToCheck, checkTimeout, healthCheckURL...)
			// Log the result of the health check.
			// In a more complex application, this might send results to a channel or use a structured logger.
			if err != nil {
				fmt.Printf("Health check for proxy '%s' (%s) - Status: %s, Error: %v\n",
					proxyToCheck.OriginalString, proxyToCheck.URL, proxyToCheck.HealthStatus, err)
			} else {
				fmt.Printf("Health check for proxy '%s' (%s) - Status: %s, Latency: %v\n",
					proxyToCheck.OriginalString, proxyToCheck.URL, proxyToCheck.HealthStatus, proxyToCheck.Latency)
			}
		}(p)
	}

	wg.Wait() // Wait for all health check goroutines to complete.
}

// GeoCheckProxy is a placeholder for future Geo-IP lookup functionality.
// Currently, it does not perform any action or modify the proxy.
//
// Parameters:
//   - proxy: A pointer to the ProxyInfo struct. Its Region field might be populated by this function in the future.
//
// Returns:
//   - `nil` (currently, as it's a placeholder).
//
// Example future use:
//  if proxy.Region == "" {
//      region, err := someGeoIPService.Lookup(proxy.URL.Hostname())
//      if err == nil { proxy.Region = region }
//  }
func GeoCheckProxy(proxy *ProxyInfo) error {
	// Placeholder: Actual Geo-IP lookup logic would be implemented here.
	// This could involve calling an external API or using a local GeoIP database.
	// For example:
	// if proxy != nil && proxy.Region == "" && proxy.URL != nil {
	//     // hostname, _, err := net.SplitHostPort(proxy.URL.Host) // More robust way to get host
	//     // if err != nil { hostname = proxy.URL.Host }
	//     // Look up 'hostname' via a Geo-IP service/database.
	//     // proxy.Region = "Fetched-Region" // Example update
	//     // fmt.Printf("Geo-check (placeholder) for proxy %s. Current region: %s\n", proxy.OriginalString, proxy.Region)
	// }
	return nil
}
