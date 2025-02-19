package auth

import (
	"k8s.io/client-go/transport"
	"net/http"
	"time"
)

// GetAuthHTTPClient returns a HTTP client which is authenticated. It can be used to query private images.
func GetAuthHTTPClient(username, password string) *http.Client {
	return &http.Client{
		Transport: transport.NewBasicAuthRoundTripper(username, password, http.DefaultTransport),
		Timeout:   30 * time.Second,
	}
}
