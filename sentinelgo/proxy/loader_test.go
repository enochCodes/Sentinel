package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a temporary CSV file for testing.
func createTempCSV(t *testing.T, content string) string {
	t.Helper()
	tempFile, err := os.CreateTemp(t.TempDir(), "test_proxies_*.csv")
	require.NoError(t, err)
	_, err = tempFile.WriteString(content)
	require.NoError(t, err)
	err = tempFile.Close()
	require.NoError(t, err)
	return tempFile.Name()
}

// Helper to create a temporary JSON file for testing.
func createTempJSON(t *testing.T, content interface{}) string {
	t.Helper()
	data, err := json.Marshal(content)
	require.NoError(t, err)
	tempFile, err := os.CreateTemp(t.TempDir(), "test_proxies_*.json")
	require.NoError(t, err)
	_, err = tempFile.Write(data)
	require.NoError(t, err)
	err = tempFile.Close()
	require.NoError(t, err)
	return tempFile.Name()
}

func TestLoadProxiesCSV(t *testing.T) {
	csvContent := `ip,port,user,pass,region
proxy1.example.com,8080,user1,pass1,US
proxy2.example.com,8000,user2,pass2,EU
proxy3.example.com,3128,,,DE
# Comment line to be skipped, or handle as error if not skipping blanks
proxy4.example.com,9090,user4,pass4
proxy5.example.com,9050
` // proxy5 has no auth and no region
	csvPath := createTempCSV(t, csvContent)
	defer os.Remove(csvPath) // Clean up

	proxies, err := LoadProxies(csvPath)
	require.NoError(t, err)
	require.Len(t, proxies, 5, "Should load 5 proxies from CSV")

	// Check proxy1
	assert.Equal(t, "http://user1:pass1@proxy1.example.com:8080", proxies[0].URL.String())
	assert.Equal(t, "proxy1.example.com,8080,user1,pass1,US", proxies[0].OriginalString)
	assert.Equal(t, csvPath, proxies[0].Source)
	assert.Equal(t, "US", proxies[0].Region)
	assert.Equal(t, "unknown", proxies[0].HealthStatus)

	// Check proxy3 (no user/pass)
	assert.Equal(t, "http://proxy3.example.com:3128", proxies[2].URL.String())
	assert.Equal(t, "proxy3.example.com,3128,,,DE", proxies[2].OriginalString)
	assert.Equal(t, "DE", proxies[2].Region)

	// Check proxy4 (no region)
    assert.Equal(t, "http://user4:pass4@proxy4.example.com:9090", proxies[3].URL.String())
	assert.Equal(t, "", proxies[3].Region)

	// Check proxy5 (no auth, no region)
	assert.Equal(t, "http://proxy5.example.com:9050", proxies[4].URL.String())
	assert.Equal(t, "proxy5.example.com,9050", proxies[4].OriginalString)
	assert.Equal(t, "", proxies[4].Region)


	// Test CSV with ip:port:user:pass:region format
	csvContentColon := `proxyhost:port:user:pass:region
proxy6.example.com:7070:user6:pass6:JP
proxy7.example.com:7001::: # No auth, no region
proxy8.example.com:7002::pass8: # No user, but pass (should probably parse as user "pass8", pass empty or handle as error)
                                 # Current parser will treat "pass8" as user, and no password for http basic auth
proxy9.example.com:7003 # No auth, no region
proxy10.example.com:7004:user10 # user, no pass, no region
`
	csvPathColon := createTempCSV(t, csvContentColon)
	defer os.Remove(csvPathColon)

	proxiesColon, err := LoadProxies(csvPathColon)
	require.NoError(t, err)
	require.Len(t, proxiesColon, 5, "Should load 5 proxies from colon-formatted CSV")

	assert.Equal(t, "http://user6:pass6@proxy6.example.com:7070", proxiesColon[0].URL.String())
	assert.Equal(t, "JP", proxiesColon[0].Region)

	assert.Equal(t, "http://proxy7.example.com:7001", proxiesColon[1].URL.String())
	assert.Equal(t, "", proxiesColon[1].Region)

	// Test proxy8 - this depends on how parseProxyString handles "user:" or ":pass"
    // Current parseProxyString might interpret "pass8" as username if only one part after @.
    // The CSV loader's specific logic for ip:port:user:pass is `userInfo := strings.Join(parts[2:4], ":")`
    // So if parts[3] is empty, it becomes "user:", if parts[2] is empty, it's ":pass"
	assert.Equal(t, "http://pass8@proxy8.example.com:7002", proxiesColon[2].URL.String()) // Assuming "pass8" becomes user if user field is empty

	assert.Equal(t, "http://proxy9.example.com:7003", proxiesColon[3].URL.String())

    assert.Equal(t, "http://user10@proxy10.example.com:7004", proxiesColon[4].URL.String())


}

func TestLoadProxiesJSON(t *testing.T) {
	jsonData := []map[string]string{
		{"proxy": "http://user1:pass1@jsonproxy1.com:8080", "region": "US"},
		{"proxy": "https://jsonproxy2.com:443"}, // No auth, no region, https scheme
		{"proxy": "jsonproxy3.com:3128", "region": "GB"}, // No scheme, no auth
	}
	jsonPath := createTempJSON(t, jsonData)
	defer os.Remove(jsonPath)

	proxies, err := LoadProxies(jsonPath)
	require.NoError(t, err)
	require.Len(t, proxies, 3, "Should load 3 proxies from JSON")

	// Check proxy1
	u1, _ := url.Parse("http://user1:pass1@jsonproxy1.com:8080")
	assert.Equal(t, u1, proxies[0].URL)
	assert.Equal(t, "http://user1:pass1@jsonproxy1.com:8080", proxies[0].OriginalString)
	assert.Equal(t, jsonPath, proxies[0].Source)
	assert.Equal(t, "US", proxies[0].Region)

	// Check proxy2
	u2, _ := url.Parse("https://jsonproxy2.com:443")
	assert.Equal(t, u2, proxies[1].URL)
	assert.Equal(t, "https://jsonproxy2.com:443", proxies[1].OriginalString)
	assert.Equal(t, "", proxies[1].Region) // No region specified

	// Check proxy3 (default scheme http)
	u3, _ := url.Parse("http://jsonproxy3.com:3128") // parseProxyString adds http if no scheme
	assert.Equal(t, u3, proxies[2].URL)
	assert.Equal(t, "jsonproxy3.com:3128", proxies[2].OriginalString)
	assert.Equal(t, "GB", proxies[2].Region)
}

func TestLoadProxies_UnsupportedFile(t *testing.T) {
	txtPath := createTempCSV(t, "this is not a proxy file") // Using CSV helper for generic temp file
	newPath := strings.Replace(txtPath, ".csv", ".txt", 1)
	err := os.Rename(txtPath, newPath)
	require.NoError(t, err)
	defer os.Remove(newPath)

	_, err = LoadProxies(newPath)
	assert.Error(t, err, "Should return an error for unsupported file types")
	assert.Contains(t, err.Error(), "unsupported proxy source format")
}

func TestLoadProxies_APIPlaceholder(t *testing.T) {
	proxies, err := LoadProxies("http://example.com/api/proxies")
	assert.NoError(t, err, "API loading placeholder should not return error currently")
	assert.Empty(t, proxies, "API loading placeholder should return empty slice")
	// TODO: Capture log output or change function to return specific signal for "not implemented"
}

func TestParseProxyString(t *testing.T) {
    tests := []struct {
        name          string
        proxyStr      string
        defaultScheme string
        expectedURL   string
        expectError   bool
    }{
        {"full_http", "http://user:pass@host.com:8080", "http", "http://user:pass@host.com:8080", false},
        {"full_https", "https://user:pass@host.com:443", "http", "https://user:pass@host.com:443", false},
        {"no_scheme_user_pass", "user:pass@host.com:80", "http", "http://user:pass@host.com:80", false},
        {"no_scheme_no_auth", "host.com:8888", "http", "http://host.com:8888", false},
        {"no_scheme_user_only", "user@host.com:80", "http", "http://user@host.com:80", false}, // url.Parse behavior
        {"no_scheme_user_colon_only", "user:@host.com:80", "http", "http://user@host.com:80", false}, // url.Parse behavior, password empty
        {"ip_port_only", "127.0.0.1:3128", "http", "http://127.0.0.1:3128", false},
        {"ip_port_user_pass", "user1:secret@192.168.1.1:8000", "http", "http://user1:secret@192.168.1.1:8000", false},
        {"empty_string", "", "http", "", true}, // url.Parse returns error on empty string
        {"invalid_url", "http://%invalid", "http", "", true},
		{"socks5_scheme", "socks5://user:pass@host.com:1080", "http", "socks5://user:pass@host.com:1080", false},
		{"no_scheme_default_socks", "user:pass@host.com:1080", "socks5", "socks5://user:pass@host.com:1080", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parsedURL, err := parseProxyString(tt.proxyStr, tt.defaultScheme)
            if tt.expectError {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expectedURL, parsedURL.String())
            }
        })
    }
}
