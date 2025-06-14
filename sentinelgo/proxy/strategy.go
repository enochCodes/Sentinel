package proxy

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Strategy constants define the available proxy selection strategies.
const (
	StrategyRoundRobin       = "round-robin"
	StrategyRandom           = "random"
	StrategyRegionPrioritized = "region-prioritized" // Note: Basic version, needs targetRegion.
)

// ErrNoHealthyProxies is returned by GetProxy when no healthy proxies are available
// and the ProxyManager is configured to only use healthy proxies.
var ErrNoHealthyProxies = errors.New("no healthy proxies available")

// ErrNoProxiesAvailable is returned by GetProxy when the proxy pool is empty.
var ErrNoProxiesAvailable = errors.New("no proxies available in the manager")

// ErrNoMatchingProxies is returned by GetProxy when no proxies match the given criteria
// (e.g., region, health status) and no fallback is available.
var ErrNoMatchingProxies = errors.New("no proxies match the specified criteria")

// ProxyManager manages a pool of proxies and implements various selection strategies.
// It allows for selecting proxies based on health, region, or rotation patterns.
// The ProxyManager is designed to be thread-safe for getting and updating proxies.
type ProxyManager struct {
	Proxies      []*ProxyInfo // The pool of all available proxies.
	currentIndex int          // Used by the round-robin strategy.
	Strategy     string       // The active proxy selection strategy (e.g., "round-robin", "random").
	HealthyOnly  bool         // If true, strategies will only consider proxies marked "healthy".
	mu           sync.Mutex   // Protects access to currentIndex and potentially the Proxies slice if it were modified dynamically post-creation.
	rng          *rand.Rand   // Local random number generator for random strategy.
}

// NewProxyManager creates and returns a new ProxyManager.
//
// Parameters:
//   - proxies: A slice of `*ProxyInfo` structs representing the initial proxy pool.
//   - strategy: A string constant (e.g., `StrategyRoundRobin`, `StrategyRandom`) specifying the
//     proxy selection strategy to use. Defaults to "round-robin" if an unknown strategy is provided.
//   - healthyOnly: A boolean indicating whether to only select from proxies marked as "healthy".
//
// The constructor initializes a local random number generator for the "random" strategy.
func NewProxyManager(proxies []*ProxyInfo, strategy string, healthyOnly bool) *ProxyManager {
	// Initialize a new random source and generator for this manager instance.
	// This avoids using the global rand which is not safe for concurrent use without locking.
	source := rand.NewSource(time.Now().UnixNano())
	localRng := rand.New(source)

	return &ProxyManager{
		Proxies:      proxies,
		Strategy:     strings.ToLower(strategy),
		HealthyOnly:  healthyOnly,
		currentIndex: 0,
		rng:          localRng,
	}
}

// GetProxy selects and returns a proxy from the pool based on the configured strategy.
//
// Parameters:
//   - targetRegion (optional): A variadic string. If the strategy is `StrategyRegionPrioritized`,
//     the first string in `targetRegion` is used as the desired region.
//
// Returns:
//   - A pointer to a `ProxyInfo` struct for the selected proxy.
//   - An error if no suitable proxy is found (e.g., pool is empty, no healthy proxies available
//     when `HealthyOnly` is true, or no proxies match region criteria). Common errors include
//     `ErrNoProxiesAvailable`, `ErrNoHealthyProxies`, `ErrNoMatchingProxies`.
//
// The method is thread-safe.
func (pm *ProxyManager) GetProxy(targetRegion ...string) (*ProxyInfo, error) {
	pm.mu.Lock() // Lock for read/write of currentIndex and for consistent view of Proxies if it were mutable.
	defer pm.mu.Unlock()

	if len(pm.Proxies) == 0 {
		return nil, ErrNoProxiesAvailable
	}

	// Filter proxies by health status if HealthyOnly is enabled.
	var candidateProxies []*ProxyInfo
	if pm.HealthyOnly {
		for _, p := range pm.Proxies {
			if p != nil && p.HealthStatus == "healthy" { // Ensure p is not nil
				candidateProxies = append(candidateProxies, p)
			}
		}
		if len(candidateProxies) == 0 {
			return nil, ErrNoHealthyProxies
		}
	} else {
		// Consider all non-nil proxies if not filtering by health.
		for _, p := range pm.Proxies {
			if p != nil {
				candidateProxies = append(candidateProxies, p)
			}
		}
	}

	if len(candidateProxies) == 0 {
		// This might happen if HealthyOnly is false but all entries in pm.Proxies were nil.
		return nil, ErrNoMatchingProxies
	}

	// Apply selection strategy.
	switch pm.Strategy {
	case StrategyRandom:
		return candidateProxies[pm.rng.Intn(len(candidateProxies))], nil

	case StrategyRegionPrioritized:
		if len(targetRegion) == 0 || targetRegion[0] == "" {
			// Fallback: If no target region specified, select randomly from available healthy/all candidates.
			return candidateProxies[pm.rng.Intn(len(candidateProxies))], nil
		}
		desiredRegion := strings.ToLower(targetRegion[0])
		var regionMatchedProxies []*ProxyInfo
		for _, p := range candidateProxies {
			if strings.EqualFold(p.Region, desiredRegion) {
				regionMatchedProxies = append(regionMatchedProxies, p)
			}
		}
		if len(regionMatchedProxies) > 0 {
			// Select randomly from proxies matching the target region.
			return regionMatchedProxies[pm.rng.Intn(len(regionMatchedProxies))], nil
		}
		// Fallback: If no proxies in the target region, select randomly from any other available candidate.
		if len(candidateProxies) > 0 { // Already checked this earlier, but good for clarity
			return candidateProxies[pm.rng.Intn(len(candidateProxies))], nil
		}
		return nil, fmt.Errorf("%w: for region '%s'", ErrNoMatchingProxies, desiredRegion)


	case StrategyRoundRobin:
		fallthrough // Default to round-robin strategy.
	default:
		// pm.currentIndex is used on candidateProxies after filtering.
		proxy := candidateProxies[pm.currentIndex%len(candidateProxies)]
		pm.currentIndex = (pm.currentIndex + 1) % len(candidateProxies) // Cycle through candidate proxies.
		return proxy, nil
	}
}

// UpdateProxyStatus updates the health status, latency, and last checked time
// of a specific proxy in the manager's list.
// The proxy is identified by its URL string.
//
// Parameters:
//   - proxyURL: The string representation of the proxy's URL to find and update.
//   - newStatus: The new health status string (e.g., "healthy", "unhealthy").
//   - latency: The latency recorded for the operation that determined this new status.
//
// Returns:
//   - `nil` if the proxy was found and updated.
//   - An error if a proxy with the given URL string is not found in the manager.
//
// The method is thread-safe.
func (pm *ProxyManager) UpdateProxyStatus(proxyURL string, newStatus string, latency time.Duration) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	found := false
	for _, p := range pm.Proxies {
		// Ensure p and p.URL are not nil before calling String()
		if p != nil && p.URL != nil && p.URL.String() == proxyURL {
			p.HealthStatus = newStatus
			p.Latency = latency
			p.LastChecked = time.Now()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("proxy with URL '%s' not found in manager for status update", proxyURL)
	}
	return nil
}

// GetAllProxies returns a new slice containing all proxies currently managed by the ProxyManager.
// This is useful for operations like batch health checks that need to iterate over all proxies.
// Returns a copy to prevent external modification of the manager's internal proxy slice.
// The method is thread-safe.
func (pm *ProxyManager) GetAllProxies() []*ProxyInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Return a copy of the slice to prevent external modifications.
	if pm.Proxies == nil {
		return nil // Or an empty slice: make([]*ProxyInfo, 0)
	}
	proxiesCopy := make([]*ProxyInfo, len(pm.Proxies))
	copy(proxiesCopy, pm.Proxies)
	return proxiesCopy
}
