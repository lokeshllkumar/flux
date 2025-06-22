package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lokeshllkumar/flux/api"
	"github.com/lokeshllkumar/flux/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// implementing the Client interface using HTTP
type httpClient struct {
	registryURL string
	httpClient  *http.Client
}

// creates a new httpClient instance
func NewHTTPClient(registryURL string, timeout time.Duration) Client {
	return &httpClient{
		registryURL: registryURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// to register the service with the service registry
func (c *httpClient) Register(ctx context.Context, instance api.ServiceInstance) error {
	opLabels := prometheus.Labels{"operation": "register", "protocol": "grpc"}
	start := time.Now()
	var status string

	defer func() {
		opLabels["status"] = status
		metrics.RegistryCallDurationSeconds.With(opLabels).Observe(time.Since(start).Seconds())
		metrics.RegistryCallsTotal.With(opLabels).Inc()
	}()

	payload, err := json.Marshal(instance)
	if err != nil {
		status = "failure"
		return fmt.Errorf("http_client: failed to marshal instance: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/services/register", c.registryURL), bytes.NewBuffer(payload))
	if err != nil {
		status = "failure"
		return fmt.Errorf("http_client: failed to create regsitration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// checking for context cancellation and timeout errors
		status = "failure"
		if ctx.Err() != nil {
			return fmt.Errorf("http_client: registration request aborted due to context: %w", ctx.Err())
		}
		return fmt.Errorf("http_client: failed to send registration request to %s: %w", c.registryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		status = "failure"
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http_client: registration failed, service registry returned non-201 status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	status = "success"
	return nil
}

// to send a heartbeat to the service registry
func (c *httpClient) SendHeartbeat(ctx context.Context, instanceID string) error {
	opLabels := prometheus.Labels{"operation": "register", "protocol": "grpc"}
	start := time.Now()
	var status string

	defer func() {
		opLabels["status"] = status
		metrics.RegistryCallDurationSeconds.With(opLabels).Observe(time.Since(start).Seconds())
		metrics.RegistryCallsTotal.With(opLabels).Inc()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/services/heartbeat/%s", c.registryURL, instanceID), nil)
	if err != nil {
		status = "failure"
		return fmt.Errorf("http_client: failed to create heartbeat request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		status = "failure"
		if ctx.Err() != nil {
			return fmt.Errorf("http_client: heartbeat request aborted due to context: %w", ctx.Err())
		}
		return fmt.Errorf("http_client: failed to send heartbeat to %s: %w", c.registryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status = "failure"
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http_client: heartbeat failed, service registry returned non-200 status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	status = "success"
	return nil
}

// to deregister the service from the service registry
func (c *httpClient) Deregister(ctx context.Context, instanceID string) error {
	opLabels := prometheus.Labels{"operation": "register", "protocol": "grpc"}
	start := time.Now()
	var status string

	defer func() {
		opLabels["status"] = status
		metrics.RegistryCallDurationSeconds.With(opLabels).Observe(time.Since(start).Seconds())
		metrics.RegistryCallsTotal.With(opLabels).Inc()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/api/v1/services/deregister/%s", c.registryURL, instanceID), nil)
	if err != nil {
		status = "failure"
		return fmt.Errorf("http_client: failed to create deregistration request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		status = "failure"
		if ctx.Err() != nil {
			return fmt.Errorf("http_client: deregistration request aborted due to context: %w", ctx.Err())
		}
		return fmt.Errorf("http_client: failed to send deregistration request to %s: %w", c.registryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		status = "failure"
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http_client: deregistration failed, service registry returned non-204 status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	status = "success"
	return nil
}

// closes the connection; exists purely to implement the interface's Close() function
func (c *httpClient) Close() error {
	return nil
}
