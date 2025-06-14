# SentinelGo++ Configuration (`config/sentinel.yaml`)

This document details the configuration options available in the `config/sentinel.yaml` file used by SentinelGo++.

The `config/sentinel.yaml` file is typically located in a `config` subdirectory relative to the application executable (e.g., `sentinelgo/config/sentinel.yaml` if running from the `sentinelgo` project root). If the file is not found upon startup, default values are used, and a new `config/sentinel.yaml` can be created by saving settings from the TUI.

**Many settings can be modified live via the "Settings" tab in the TUI and saved back to this file.**

## Main Configuration Fields

### `defaultheaders`
*   **Type**: `map[string]string`
*   **Description**: A map of HTTP headers that will be included in every report request by default. These are standard HTTP headers.
*   **Example**:
    ```yaml
    defaultheaders:
      User-Agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
      Accept-Language: "en-US,en;q=0.9"
      Referer: "https://www.tiktok.com/" # Example
    ```

### `customcookies`
*   **Type**: `array` of cookie objects (maps)
*   **Description**: A list of custom cookies to be added to report requests. Each cookie object should conform to the structure expected by Go's `net/http.Cookie` when unmarshaled, but in YAML format. Key fields include:
    *   `name`: The name of the cookie.
    *   `value`: The value of the cookie.
    *   `path`: (Optional) The path scope of the cookie.
    *   `domain`: (Optional) The domain scope of the cookie.
    *   `expires`: (Optional) Expiry timestamp in RFC3339 format (e.g., `2024-12-31T23:59:59Z`).
    *   `httponly`: (Optional) Boolean, true if HTTPOnly.
    *   `secure`: (Optional) Boolean, true if Secure flag should be set.
*   **Example**:
    ```yaml
    customcookies:
      - name: "session_id_example"
        value: "dummycookie123abcXYZ"
        path: "/"
        domain: ".example.com"
        # expires: "2024-12-31T23:59:59Z"
        # httponly: true
        # secure: true
      - name: "another_tracker"
        value: "value_for_tracker"
    ```

### `maxretries`
*   **Type**: `int`
*   **Description**: The maximum number of times a single report attempt (one of the "Number of Reports") will be retried if it fails due to network issues or specific error responses. This is handled by the `report.Reporter`.
*   **Default (if file not found or key missing)**: 3
*   **TUI Editable**: Yes

### `riskthreshold`
*   **Type**: `float`
*   **Description**: A percentage threshold (0.0 - 100.0) for the AI content analyzer. If the AI's calculated threat score for a piece of content exceeds this threshold, a "high risk" event is logged, and potentially other actions could be triggered in the future.
*   **Default (if file not found or key missing)**: 75.0
*   **TUI Editable**: Yes

### `apikeys`
*   **Type**: `map[string]string`
*   **Description**: A map to store API keys for various external services. This allows centralizing API key management. For instance, if a real AI analysis service is integrated, its key would go here.
*   **Example**:
    ```yaml
    apikeys:
      virustotal_example: "YOUR_VT_API_KEY_HERE"
      ai_service_provider: "AI_SERVICE_KEY_XYZ123"
    ```
    *(Currently, these are placeholders and not used by the `DummyAnalyzer`.)*

## Proxy Configuration Notes

*   **Proxy Source:** The primary way to load proxies is by providing a CSV or JSON file. The path to this file is currently hardcoded in `tui/model.go` as a default (`config/proxies.csv`) but can be notionally overridden by setting `DefaultHeaders.ProxyFile` in `sentinel.yaml` (this is an example of how it *could* be configured, though the TUI doesn't yet offer editing for this specific header for this purpose).
    *   Example (conceptual, if `ProxyFile` header was used for this):
        ```yaml
        defaultheaders:
          ProxyFile: "config/my_custom_proxies.json"
        ```
*   **Proxy File Formats:**
    *   **CSV:** Lines in `ip,port,user,pass[,region]` or `ip:port:user:pass[:region]` format.
    *   **JSON:** An array of objects, each with a `"proxy"` string (e.g., "http://user:pass@host:port") and an optional `"region"` string.
*   Refer to `docs/USER_GUIDE.md` for more on how proxies are used.

## Example `config/sentinel.yaml`

```yaml
# Default Application Settings for SentinelGo++
# Refer to docs/CONFIGURATION.md for detailed explanations of each field.

defaultheaders:
  User-Agent: "SentinelGo Client v1.0 (Compatible; MSIE 9.0; Windows NT 6.1; Trident/5.0)"
  Accept-Language: "en-US,en;q=0.9,es;q=0.8"
  # ProxyFile: "config/proxies.json" # Conceptual: if proxy file path were set here

customcookies:
  - name: "user_preference"
    value: "darkMode=true; notifications=off"
    path: "/"
  - name: "language_selected"
    value: "en-US"

maxretries: 3

riskthreshold: 80.5 # Stricter than default

apikeys:
  # Dummy keys, replace with actual keys if integrating real services
  service_alpha: "alpha_key_placeholder_12345"
  service_beta: "beta_key_placeholder_67890"

```

*(This document will be updated as more configuration options are added or refined. Always check the latest version for accurate information.)*
