package utils

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// MeasureLatency tests the network latency to a given base URL.
func MeasureLatency(baseURL string, attempts int) (float64, error) {
	if baseURL == "" {
		return 0, fmt.Errorf("empty base URL")
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return 0, fmt.Errorf("invalid base URL: %w", err)
	}

	var totalLatency float64
	for i := 0; i < attempts; i++ {
		start := time.Now()
		conn, err := http.Get(parsedURL.Scheme + "://" + parsedURL.Host)
		if err != nil {
			return 0, fmt.Errorf("HTTP GET error: %w", err)
		}
		conn.Body.Close()
		totalLatency += float64(time.Since(start).Milliseconds())
	}
	return totalLatency / float64(attempts), nil
}
