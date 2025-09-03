package probes

import "context"

type maintenanceCtxKey struct{}

func withMaintenance(ctx context.Context, on bool) context.Context {
	return context.WithValue(ctx, maintenanceCtxKey{}, on)
}

func maintenanceFromContext(ctx context.Context) bool {
	if v, ok := ctx.Value(maintenanceCtxKey{}).(bool); ok {
		return v
	}
	return false
}
