package registration

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lokeshllkumar/flux/api"
	"github.com/lokeshllkumar/flux/metrics"
	"github.com/lokeshllkumar/flux/registry"
)

// config for the registrar
type Config struct {
	RegistryURL       string
	RegistryType  string
	HeartbeatInterval time.Duration
	CallTimeout       time.Duration
	MaxRetries        int
	RetryDelay        time.Duration
}

// returns a new Config with defaults
func NewDefaultConfig() *Config {
	return &Config{
		HeartbeatInterval: 10 * time.Second,
		CallTimeout: 5 * time.Second,
		MaxRetries: 5,
		RetryDelay: 1 * time.Second,
	}
}

// manages service instance's lifecycle with the registry
type Registrar struct {
	instance api.ServiceInstance
	client registry.Client
	config *Config
	wg sync.WaitGroup
	stopHeartbeat chan struct{}
}

// creates a new Registrar instance
func NewRegistrar(instance api.ServiceInstance, cfg *Config) (*Registrar, error) {
	if cfg == nil {
		return nil, fmt.Errorf("registration: config cannot be nil")
	}
	if cfg.RegistryURL == "" {
		return nil, fmt.Errorf("registration: RegistryURL must be provided in the config")
	}
	if cfg.HeartbeatInterval <= 0 {
		return nil, fmt.Errorf("registration: HeartbeatInterval must be a positive duration")
	}
	if cfg.CallTimeout <= 0 {
		return nil, fmt.Errorf("registration: CallTimeout must be a positive duration")
	}
	if cfg.MaxRetries < 0 {
		return nil, fmt.Errorf("registration: MaxRetries must be non-negative")
	}
	if cfg.RetryDelay < 0 {
		return nil, fmt.Errorf("registration: RetryDelay must be non-negative")
	}

	var client registry.Client
	var err error

	switch cfg.RegistryType {
	case "http":
		client = registry.NewHTTPClient(cfg.RegistryURL, cfg.CallTimeout)
	
	case "grpc":
		client, err = registry.NewGRPCClient(cfg.RegistryURL, cfg.CallTimeout)
		if err != nil {
			return nil, fmt.Errorf("registration: failed to create gRPC registry client: %w", err)
		}
	default:
		return nil, fmt.Errorf("registration: unsupported registry client type'%s'. Must be 'http' or 'grpc'", cfg.RegistryType)
	}

	return &Registrar{
		instance: instance,
		client: client,
		config: cfg,
		stopHeartbeat: make(chan struct{}),
	}, nil
}

// initiates the auto-registration process
// performs initial registration and starts a gorouting to perform periodic heartbeats
func (r *Registrar) Start(ctx context.Context) {
	log.Printf("Registration: Attempting initial registration for service '%s' (ID: %s)...", r.instance.ServiceName, r.instance.ID)
	err := r.registerWithRetry(ctx)
	if err != nil {
		log.Printf("Registration: Initial registration for '%s' (ID: %s) failed after retries: %v", r.instance.ServiceName, r.instance.ID, err)
		metrics.RegistrarStateGauge.WithLabelValues(r.instance.ID, r.instance.ServiceName).Set(0)
	} else {
		log.Printf("Registration: Service '%s' (ID: %s) successfully registered", r.instance.ServiceName, r.instance.ID)
		metrics.RegistrarStateGauge.WithLabelValues(r.instance.ID, r.instance.ServiceName).Set(1)
	}

	r.wg.Add(1)
	go r.runHeartbeatLoop(ctx)
}

// sends periodic heartbeats to the service registry for health checks and attempts re-registration of the service in the event of a heartbeat failure
func (r *Registrar) runHeartbeatLoop(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <- ticker.C:
			heartbeatCtx, cancel := context.WithTimeout(ctx, r.config.CallTimeout)
			err := r.client.SendHeartbeat(heartbeatCtx, r.instance.ID)
			cancel()

			if err != nil {
				log.Printf("Heartbeat failed for '%s' (ID: %s): %v. Attempting to re-register...", r.instance.ServiceName, r.instance.ID, err)
				registrationErr := r.registerWithRetry(ctx)
				if registrationErr != nil {
					log.Printf("Re-registration after heartbeat failure failed for '%s' (ID %s): %v", r.instance.ServiceName, r.instance.ID, err)
					metrics.RegistrarStateGauge.WithLabelValues(r.instance.ID, r.instance.ServiceName).Set(0)
				} else {
					log.Printf("Service '%s' (ID: %s) successfully re-registered after heartbeat failure", r.instance.ServiceName, r.instance.ID)
					metrics.RegistrarStateGauge.WithLabelValues(r.instance.ID, r.instance.ServiceName).Set(1)
				}
			} else {
				log.Printf("Heartbeat sent for service'%s' (ID: %s)", r.instance.ServiceName, r.instance.ID)
				metrics.RegistrarStateGauge.WithLabelValues(r.instance.ID, r.instance.ServiceName).Set(1)
			}
		case <- r.stopHeartbeat:
			log.Printf("Heartbeat loop for '%s' stopped", r.instance.ID)
			return
		case <- ctx.Done():
			log.Printf("Heartbeat loop for '%s' stopped due to context cancellation", r.instance.ServiceName)
		}
	}
}

func (r *Registrar) registerWithRetry(ctx context.Context) error {
	for i := 0; i < r.config.MaxRetries; i++ {
		// fresh context used for each retry
		callCtx, cancel := context.WithTimeout(ctx, r.config.CallTimeout)
		err := r.client.Register(callCtx, r.instance)
		cancel()

		if err == nil {
			return nil
		}

		log.Printf("Registration attempt %d/%d failed for '%s' (ID: %s): %v. Retrying in %v...",
					i + 1, r.config.MaxRetries, r.instance.ServiceName, r.instance.ID, err, r.config.RetryDelay * (time.Duration(1 << i))) // exponentially increasing wait time to not overwhelm the service registry
		
		select {
		case <- time.After(r.config.RetryDelay * time.Duration(1 << i)):
			// next retry
		case <- ctx.Done():
			return fmt.Errorf("registration: aborted retry for '%s' due to context cancellation: %w", r.instance.ServiceName, ctx.Err())
		}
	}
	return fmt.Errorf("registration: failed to regsiter service '%s' (ID: %s) after %d retries", r.instance.ServiceName, r.instance.ID, r.config.MaxRetries)
}

// initiates the graceful deregistering of the service and stops ongoing heartbeats
func (r *Registrar) Stop(ctx context.Context) {
	log.Printf("Registration: Initiating graceful shutdown for service '%s' (ID : %s)...", r.instance.ServiceName, r.instance.ID)

	close(r.stopHeartbeat)
	r.wg.Wait()

	deregistrationContext, cancel:= context.WithTimeout(ctx, r.config.CallTimeout)
	defer cancel()

	if err := r.client.Deregister(deregistrationContext, r.instance.ID); err != nil {
		log.Printf("Registration: Deregistration failed for '%s' (ID: %s): %v", r.instance.ServiceName, r.instance.ID, err)
	} else {
		log.Printf("Registration: Service '%s' (ID: %s) successfully deregistered", r.instance.ServiceName, r.instance.ID)
	}

	if err := r.client.Close(); err != nil {
		log.Printf("Registration: Failed to close registry client connection: %v", err)
	}
	metrics.RegistrarStateGauge.WithLabelValues(r.instance.ID, r.instance.ServiceName).Set(0)
	log.Println("Registration: Registrar stopped")
}