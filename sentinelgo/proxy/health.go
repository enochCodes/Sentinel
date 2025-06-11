package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const defaultHealthCheckURL = "http://httpbin.org/get" // A reliable endpoint for checking proxy connectivity

// CheckProxyHealth attempts to make a lightweight HTTP GET request through the given proxy.
// It updates the proxy's HealthStatus, Latency, and LastChecked fields.
func CheckProxyHealth(proxy *ProxyInfo, timeout time.Duration, healthCheckURL ...string) error {
	checkURL := defaultHealthCheckURL
	if len(healthCheckURL) > 0 && healthCheckURL[0] != "" {
		checkURL = healthCheckURL[0]
	}

	if proxy.URL == nil {
		proxy.HealthStatus = "unhealthy"
		proxy.LastChecked = time.Now()
		return fmt.Errorf("proxy %s has nil URL", proxy.OriginalString)
	}

	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxy.URL)},
		Timeout:   timeout,
	}

	startTime := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), "GET", checkURL, nil)
	if err != nil {
		proxy.HealthStatus = "unhealthy"
		proxy.LastChecked = time.Now()
		return fmt.Errorf("failed to create request for proxy %s: %w", proxy.OriginalString, err)
	}

	resp, err := client.Do(req)
	proxy.LastChecked = time.Now()
	proxy.Latency = time.Since(startTime)

	if err != nil {
		proxy.HealthStatus = "unhealthy"
		return fmt.Errorf("proxy %s request failed (%s): %w", proxy.OriginalString, checkURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		proxy.HealthStatus = "healthy"
	} else {
		proxy.HealthStatus = "unhealthy"
		return fmt.Errorf("proxy %s request to %s returned status %d", proxy.OriginalString, checkURL, resp.StatusCode)
	}

	return nil
}

// BatchCheckProxies concurrently checks the health of a list of proxies.
func BatchCheckProxies(proxies []*ProxyInfo, checkTimeout time.Duration, concurrency int, healthCheckURL ...string) {
	if concurrency <= 0 {
		concurrency = 1 // Ensure at least one worker
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency) // Limit concurrent goroutines

	for _, p := range proxies {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire a slot

		go func(proxyToCheck *ProxyInfo) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release slot

			err := CheckProxyHealth(proxyToCheck, checkTimeout, healthCheckURL...)
			if err != nil {
				// Suppress error logs for proxy health check failures in CLI window.
				// Optionally, log to file or metrics here if needed.
				// fmt.Printf("Health check for %s (%s) - Status: %s, Error: %v\n", proxyToCheck.OriginalString, proxyToCheck.URL, proxyToCheck.HealthStatus, err)
			} else {
				fmt.Printf("Health check for %s (%s) - Status: %s, Latency: %v\n", proxyToCheck.OriginalString, proxyToCheck.URL, proxyToCheck.HealthStatus, proxyToCheck.Latency)
			}
		}(p)
	}

	wg.Wait()
}

// GeoCheckProxy is a placeholder for future Geo-IP lookup functionality.
// For now, it does nothing or can return a dummy region if proxy.Region is empty.
func GeoCheckProxy(proxy *ProxyInfo) error {
	// Placeholder: Actual Geo-IP lookup would go here.
	// For example, using a service or a local database.
	// if proxy.Region == "" {
	// proxy.Region = "US-Placeholder" // Example
	// }
	// fmt.Printf("Geo-check (placeholder) for proxy %s. Current region: %s\n", proxy.OriginalString, proxy.Region)
	return nil
}
