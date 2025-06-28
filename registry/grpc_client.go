package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/lokeshllkumar/flux/api"
	pb "github.com/lokeshllkumar/flux/gen"
	"github.com/lokeshllkumar/flux/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// implementing the client interface using gRPC
type grpcClient struct {
	registryAddress string
	client          pb.ServiceRegistryClient
	conn            *grpc.ClientConn
}

// creates a new instance of grpcClient
func NewGRPCClient(registryAddress string, timeout time.Duration) (Client, error) {
	// establish gRPC connection
	conn, err := grpc.NewClient(
		registryAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc_client: failed to create gRPC client connection for %s: %w", registryAddress, err)
	}

	client := pb.NewServiceRegistryClient(conn)

	return &grpcClient{
		registryAddress: registryAddress,
		client:          client,
		conn:            conn,
	}, nil
}

// checks if the gRPC connection is ready for use
func (c *grpcClient) ensureConnectionReady(ctx context.Context) error {
	state := c.conn.GetState()
	if state == connectivity.Ready {
		return nil
	}

	// makes it wait until it reaches a ready state
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// blocks until the state changes or the context is done
	c.conn.WaitForStateChange(waitCtx, state)

	if c.conn.GetState() != connectivity.Ready {
		if ctx.Err() != nil {
			return fmt.Errorf("grpc_client: connection to registry not ready, and original context cancelled: %w. Current state: %s", ctx.Err(), c.conn.GetState().String())
		}
		return fmt.Errorf("grpc_client: connection to service registry is not ready for RPC. Current state: %s", c.conn.GetState().String())
	}
	return nil
}

// to register the service with the service registry
func (c *grpcClient) Register(ctx context.Context, instance api.ServiceInstance) error {
	opLabels := prometheus.Labels{"operation": "register", "protocol": "grpc"}
	start := time.Now()
	var status string

	defer func() {
		opLabels["status"] = status
		metrics.RegistryCallDurationSeconds.With(opLabels).Observe(time.Since(start).Seconds())
		metrics.RegistryCallsTotal.With(opLabels).Inc()
	}()

	if err := c.ensureConnectionReady(ctx); err != nil {
		status = "failure"
		return fmt.Errorf("grpc_client: connection not ready for registration: %w", err)
	}

	grpcInstance := &pb.GrpcServiceInstance{
		Id:          instance.ID,
		ServiceName: instance.ServiceName,
		Host:        instance.Host,
		Port:        int32(instance.Port),
		Url:         instance.URL,
		HealthPath:  instance.HealthPath,
	}

	req := &pb.RegisterServiceRequest{
		Instance: grpcInstance,
	}
	resp, err := c.client.RegisterService(ctx, req)
	if err != nil {
		status = "failure"
		return fmt.Errorf("grpc_client: registration failed: %w", err)
	}
	if !resp.GetSuccess() {
		status = "failure"
		return fmt.Errorf("grpc_client: registration failed, service registry response: %s", resp.GetMessage())
	}

	status = "success"
	return nil
}

// sends gRPC request to service registry to update its heartbeat
func (c *grpcClient) SendHeartbeat(ctx context.Context, instanceID string) error {
	opLabels := prometheus.Labels{"operation": "register", "protocol": "grpc"}
	start := time.Now()
	var status string

	defer func() {
		opLabels["status"] = status
		metrics.RegistryCallDurationSeconds.With(opLabels).Observe(time.Since(start).Seconds())
		metrics.RegistryCallsTotal.With(opLabels).Inc()
	}()

	if err := c.ensureConnectionReady(ctx); err != nil {
		status = "failure"
		return fmt.Errorf("grpc_client: connection not ready for heartbeat: %w", err)
	}

	req := &pb.SendHeartbeatRequest{
		InstanceId: instanceID,
	}
	resp, err := c.client.SendHeartbeat(ctx, req)
	if err != nil {
		status = "failure"
		return fmt.Errorf("grpc_client: heartbeat failed for %s: %w", instanceID, err)
	}
	if !resp.GetSuccess() {
		status = "failure"
		return fmt.Errorf("grpc_client: heartbeat failed, registry response: %s", resp.GetMessage())
	}

	status = "success"
	return nil

}

// to deregister the service from the service registry
func (c *grpcClient) Deregister(ctx context.Context, instanceID string) error {
	opLabels := prometheus.Labels{"operation": "register", "protocol": "grpc"}
	start := time.Now()
	var status string

	defer func() {
		opLabels["status"] = status
		metrics.RegistryCallDurationSeconds.With(opLabels).Observe(time.Since(start).Seconds())
		metrics.RegistryCallsTotal.With(opLabels).Inc()
	}()

	if err := c.ensureConnectionReady(ctx); err != nil {
		status = "failure"
		return fmt.Errorf("grpc_client: connection not ready for deregistration: %w", err)
	}

	req := &pb.DeregisterServiceRequest{
		InstanceId: instanceID,
	}
	resp, err := c.client.DeregisterService(ctx, req)
	if err != nil {
		status = "failure"
		return fmt.Errorf("grpc_client: deregistration failed for %s: %w", instanceID, err)
	}
	if !resp.GetSuccess() {
		status = "failed"
		return fmt.Errorf("grpc_client: deregistration failed, service registry response: %s", resp.GetMessage())
	}

	status = "success"
	return nil
}

// queries the service registry for a lit of healthy instances of a service
func (c *grpcClient) GetHealthyServices(ctx context.Context, serviceName string) ([]api.ServiceInstance, error) {
	opLabels := prometheus.Labels{"operation": "get_healthy_services", "protocol": "grpc"}
	start := time.Now()
	var status string
	defer func() {
		opLabels["status"] = status
		metrics.RegistryCallDurationSeconds.With(opLabels).Observe(time.Since(start).Seconds())
		metrics.RegistryCallsTotal.With(opLabels).Inc()
	}()

	if err := c.ensureConnectionReady(ctx); err != nil {
		status = "failure"
		return nil, fmt.Errorf("grpc_client: connection not ready for get_healthy_services: %w", err)
	}

	req := &pb.GetHealthyServicesRequest{InstanceName: serviceName}
	resp, err := c.client.GetHealthyServices(ctx, req)
	if err != nil {
		status = "failure"
		return nil, fmt.Errorf("grpc_client: failed to get healthy services for %s: %w", serviceName, err)
	}

	var instances []api.ServiceInstance
	for _, grpcInstance := range resp.GetInstances() {
		instances = append(instances, api.ServiceInstance{
			ID: grpcInstance.GetId(),
			ServiceName: grpcInstance.GetServiceName(),
			Host: grpcInstance.GetHost(),
			Port: int(grpcInstance.GetPort()),
			URL: grpcInstance.GetUrl(),
			HealthPath: grpcInstance.GetHealthPath(),
		})
	}

	status = "success"
	return instances, nil
}

// closes the gRPC client connection; use to release resources when the service is shutting down
func (c *grpcClient) Close() error {
	if c.conn != nil {
		if c.conn.GetState() != connectivity.Shutdown {
			return c.conn.Close()
		}
	}

	return nil
}
