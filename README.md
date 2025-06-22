# flux

A minimal Go framework to simplify to simplify the interaction of your Go backend services with an external *Service Registry*. Built in tandem with the [Load Balancer project](https://github.com/lokeshllkumar/load-balancer), Flux provides a structured, opinionated way to automatically register and manage your service's presence within a *Service Registry*, offering pre-defined interfaces, built-in logic for connection retries and heartbeats, and clear integration points for observability. By abstracting the complexities of service registration, heartbeat management, and deregistration, allowing your services to focus on their core business logic.

## Features

- Automated Service Registration: Registers your Go backend services with the service registry upon server startup
- Healthchecks: Automatically sends heartbeats to maintain the service's activity status
- Graceful Degradation: Attempts to deregister the service upon shutdown
- Configurable Registry Clients: Supports both HTTP/REST and gRPC communication
- Observability: Exposes detaled metrics on registry calls (such as latency) and service instance health.

## Components

The module is structured into the following logical packages:
- [```api```](api/) - Defines a data structure, ```ServiceInstance```, which represents a specific instance of a backend service
- [```metrics```](metrics/) - Provides Prometheus metric definitions and an HTTP handler for exposition of scraped metrics
- [```registry```](registry/) - Defines the ```Client``` interface with implementations for both, HTTP and gRPC
- [```registration```](registration/) - Contains ```Registrar```, which orchestrates the service lifecycle with the service registry

## Getting Started

To use in the Flux in your Go's backend service, follow these instructions to get started:

- Add ```flux``` to your project's ```go.mod``` file
```bash
go get github.com/lokeshllkumar/flux
go mod tidy
```

- If you intend to use gRPC, generate protobof stubs using the ```service_registry.proto``` file in the [```proto```](proto/) directory by running the following command:
```bash
# from within flux/proto/
protoc --go_out=../gen --go_opt=paths=source_relative \
       --go-grpc_out=../gen --go-grpc_opt=paths=source_relative \
       service_registry.proto
```
- To scrape metrics for your backend, add the following to your ```prometheus.yml``` config file for Prometheus
```yaml
scrape_configs:
  - job_name: 'go_backend_service'
    static_configs:
    # add each service instance's metrics endpoint as a unique target for Prometheus to scrape metrics from
      - targets: ['instance-1-address']
      - targets: ['instance-2-address']
    metrics_path: /metrics
```

## Usage

- Integrate into your Go backend service (a simple reference can be found [here](https://github.com/lokeshllkumar/flux/tree/main/examples/go-backend-service))
- Configure the backend service by setting up the following environment variables
    - ```SERVICE_NAME```: A logical name for your service
    - ```PORT``` - The main application's port number
    - ```METRICS_PORT``` - The port number via which your service's Prometheus metrics are exposed
    - ```REGISTRY_URL``` - The service registry's address
    - ```HOSTNAME``` - The service's reachable host/IP