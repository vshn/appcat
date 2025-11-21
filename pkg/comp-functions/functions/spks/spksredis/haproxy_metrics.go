package spksredis

import (
	"context"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	helmv1beta1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	spksv1alpha1 "github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func HaproxyMetrics(ctx context.Context, comp *spksv1alpha1.CompositeRedisInstance, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to parse composite: %w", err))
	}

	release := &helmv1beta1.Release{}
	err = svc.GetDesiredComposedResourceByName(release, redisRelease)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get redis release from desired iof: %w", err))
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot parse release values from desired release: %w", err))
	}

	haproxyExtraConfig := fmt.Sprintf(`
backend filterproxy
  mode http
  http-request set-query namespace=%s
  server filter exporter-filterproxy.syn-exporter-filterproxy.svc.cluster.local:8080

frontend redisMetrics
  mode http
  bind *:9090
  option httplog
  use_backend redis-node-metrics-0 if { path_beg /redis/0 }
  use_backend redis-node-metrics-1 if { path_beg /redis/1 }
  use_backend redis-node-metrics-2 if { path_beg /redis/2 }
  use_backend filterproxy

backend redis-node-metrics-0
  mode http
  http-request set-path /metrics
  server node-0 redis-announce-0:9121 init-addr none check
backend redis-node-metrics-1
  mode http
  http-request set-path /metrics
  server node-1 redis-announce-1:9121 init-addr none check
backend redis-node-metrics-2
  mode http
  http-request set-path /metrics
  server node-2 redis-announce-2:9121 init-addr none check`, comp.GetName())

	if err := common.SetNestedObjectValue(values, []string{"haproxy", "extraConfig"}, haproxyExtraConfig); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set helm values: %w", err))
	}

	vb, err := json.Marshal(values)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot marshal release values: %w", err))
	}

	release.Spec.ForProvider.Values.Raw = vb

	if err := svc.SetDesiredComposedResourceWithName(release, redisRelease); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set new release: %w", err))
	}

	return nil
}
