package registry

import (
	"context"
	"github.com/lokeshllkumar/flux/api"
)

type Client interface {
	Register(ctx context.Context, instance api.ServiceInstance) error
	SendHeartbeat(ctx context.Context, instanceID string) error
	Deregister(ctx context.Context, instanceID string) error
	Close() error
}