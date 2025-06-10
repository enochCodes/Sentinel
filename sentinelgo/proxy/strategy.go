package proxy

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// ErrNoHealthyProxies is returned when no healthy proxies are available.
var ErrNoHealthyProxies = errors.New("no healthy proxies available")

// ProxyManager manages a pool of proxies and implements selection strategies.
type ProxyManager struct {
	Proxies      []*ProxyInfo
	currentIndex int
	Strategy     string
	HealthyOnly  bool
	mu           sync.Mutex // For thread-safe operations on currentIndex and Proxies list if needed
}

// NewProxyManager creates a new ProxyManager.
func NewProxyManager(proxies []*ProxyInfo, strategy string, healthyOnly bool) *ProxyManager {
	// Seed random number generator for random strategy
	rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:staticcheck // SA1019: rand.Seed has been deprecated for global random number generator, but okay for local.

	return &ProxyManager{
		Proxies:      proxies,
		Strategy:     strings.ToLower(strategy),
		HealthyOnly:  healthyOnly,
		currentIndex: 0,
	}
}

// GetProxy selects a proxy based on the configured strategy.
// For "region-prioritized", it expects one optional argument: targetRegion (string).
func (pm *ProxyManager) GetProxy(targetRegion ...string) (*ProxyInfo, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if len(pm.Proxies) == 0 {
		return nil, errors.New("no proxies available in the manager")
	}

	var availableProxies []*ProxyInfo
	if pm.HealthyOnly {
		for _, p := range pm.Proxies {
			if p.HealthStatus == "healthy" {
				availableProxies = append(availableProxies, p)
			}
		}
		if len(availableProxies) == 0 {
			return nil, ErrNoHealthyProxies
		}
	} else {
		availableProxies = pm.Proxies
	}
    if len(availableProxies) == 0 {
        return nil, errors.New("no proxies match criteria (e.g. all unhealthy or list empty)")
    }


	switch pm.Strategy {
	case "random":
		return availableProxies[rand.Intn(len(availableProxies))], nil
	case "region-prioritized":
		if len(targetRegion) == 0 || targetRegion[0] == "" {
			// Fallback to round-robin or random if no region specified
			// For simplicity, falling back to selecting any healthy proxy randomly
			if len(availableProxies) == 0 { // Should be caught by HealthyOnly check earlier
				return nil, ErrNoHealthyProxies
			}
			return availableProxies[rand.Intn(len(availableProxies))], nil
		}
		desiredRegion := targetRegion[0]
		var regionMatchedProxies []*ProxyInfo
		for _, p := range availableProxies {
			if strings.EqualFold(p.Region, desiredRegion) {
				regionMatchedProxies = append(regionMatchedProxies, p)
			}
		}
		if len(regionMatchedProxies) > 0 {
			return regionMatchedProxies[rand.Intn(len(regionMatchedProxies))], nil // Random from matched region
		}
		// Fallback: if no proxies in the target region, return any available proxy (randomly)
		if len(availableProxies) > 0 {
			return availableProxies[rand.Intn(len(availableProxies))], nil
		}
		return nil, fmt.Errorf("no proxies available for region %s, and no fallback proxies found", desiredRegion)
	case "round-robin":
		fallthrough // Default to round-robin
	default:
		if len(availableProxies) == 0 { // Should be caught by HealthyOnly check earlier
             return nil, ErrNoHealthyProxies
        }
		// Round-robin logic
		proxy := availableProxies[pm.currentIndex%len(availableProxies)]
		pm.currentIndex = (pm.currentIndex + 1) % len(availableProxies)
		return proxy, nil
	}
}

// UpdateProxyStatus updates the health status and latency of a specific proxy in the manager's list.
// This is useful if an external operation determines a proxy's status has changed.
func (pm *ProxyManager) UpdateProxyStatus(proxyURL string, newStatus string, latency time.Duration) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for _, p := range pm.Proxies {
		if p.URL.String() == proxyURL {
			p.HealthStatus = newStatus
			p.Latency = latency
			p.LastChecked = time.Now()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("proxy with URL %s not found in manager", proxyURL)
	}
	return nil
}

//GetAllProxies returns all proxies managed by the ProxyManager, useful for batch health checks after initialization.
func (pm *ProxyManager) GetAllProxies() []*ProxyInfo {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    // Return a copy to prevent external modification of the slice
    proxiesCopy := make([]*ProxyInfo, len(pm.Proxies))
    copy(proxiesCopy, pm.Proxies)
    return proxiesCopy
}
