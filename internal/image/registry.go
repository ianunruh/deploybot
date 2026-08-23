package image

import (
	"net/http"
	"time"
)

// DefaultRegistry lists GHCR and Docker Hub with a shared HTTP client.
func DefaultRegistry() *Registry {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	gh := NewGitHub()
	gh.HTTPClient = httpClient
	hub := NewDockerHub()
	hub.HTTPClient = httpClient
	return &Registry{GitHub: gh, DockerHub: hub}
}
