package slareport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/appuio/appuio-cloud-reporting/pkg/thanos"
	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	metricQuery = `(1 - max without(prometheus_replica) (sum_over_time(slo:sli_error:ratio_rate5m{sloth_service=~"appcat-.+"}[{{DURATION}}])
/ ignoring (sloth_window) count_over_time(slo:sli_error:ratio_rate5m{sloth_service=~"appcat-.+"}[{{DURATION}}])
) >= 0) * 100`
	slaQuery         = `slo:objective:ratio{sloth_id="{{SLOTHID}}"}`
	promClientFunc   = getPrometheusAPIClient
	getMetricsFunc   = getSLAMetrics
	getTargetSLAFunc = getTargetSLA
)

func getPrometheusAPIClient(promURL string, thanosAllowPartialResponses bool, orgID string) (apiv1.API, error) {
	rt := api.DefaultRoundTripper
	rt = &thanos.PartialResponseRoundTripper{
		RoundTripper: rt,
		Allow:        thanosAllowPartialResponses,
	}

	if orgID != "" {
		rt = &thanos.AdditionalHeadersRoundTripper{
			RoundTripper: rt,
			Headers: map[string][]string{
				"X-Scope-OrgID": {orgID},
			},
		}
	}

	client, err := api.NewClient(api.Config{
		Address:      promURL,
		RoundTripper: rt,
	})

	return apiv1.NewAPI(client), err
}

func RunQuery(ctx context.Context, promURL, timeRange, date, mimirOrg string) (map[string][]ServiceInstance, error) {

	l := log.FromContext(ctx)

	l.Info("Starting SLA queries", "date", date, "range", timeRange)

	client, err := promClientFunc(promURL, true, mimirOrg)
	if err != nil {
		return nil, err
	}

	l.V(1).Info("Parsing timerange", "date", date, "range", timeRange)
	startDate, endDate, err := parseRange(date, timeRange)
	if err != nil {
		return nil, fmt.Errorf("cannot parse date or timeRange: %w", err)
	}

	l.V(1).Info("Querying prometheus", "promURL", promURL)
	samples, err := getMetricsFunc(ctx, startDate, endDate, timeRange, client)
	if err != nil {
		return nil, fmt.Errorf("error during metrics prometheus query: %w", err)
	}

	slaMetrics := map[string][]ServiceInstance{}

	for _, sample := range samples {

		org := string(sample.Metric["organization"])
		if org == "" {
			org = "noOrganizationInfo"
		}

		l.V(1).Info("Parsing metrics", "org", org)

		slothID := string(sample.Metric["sloth_id"])

		targetSLA, err := getTargetSLAFunc(ctx, slothID, client, endDate)
		if err != nil {
			return nil, fmt.Errorf("error during SLA target query: %w", err)
		}

		if len(sample.Values) == 0 {
			return slaMetrics, errors.New("no values available for instance " + string(sample.Metric["name"]))
		}

		outcomeSLA := float64(sample.Values[len(sample.Values)-1].Value)
		color := "green"
		if targetSLA > outcomeSLA {
			l.V(1).Info("SLA not reached", "target", targetSLA, "outcome", outcomeSLA)
			color = "red"
		}

		slaMetrics[org] = append(slaMetrics[org], ServiceInstance{
			Instance:   string(sample.Metric["name"]),
			Namespace:  string(sample.Metric["namespace"]),
			OutcomeSLA: outcomeSLA,
			TargetSLA:  targetSLA,
			Service:    string(sample.Metric["service"]),
			Cluster:    string(sample.Metric["cluster_id"]),
			Color:      color,
		})
	}

	return slaMetrics, nil
}

func parseRange(date, duration string) (*time.Time, *time.Time, error) {
	endDate, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return nil, nil, err
	}

	parsedRange, err := model.ParseDuration(duration)
	if err != nil {
		return nil, nil, err
	}

	parsedDuration := time.Duration(parsedRange)

	startDate := endDate.Add(-1 * parsedDuration)

	return &startDate, &endDate, nil
}

func getSLAMetrics(ctx context.Context, startDate, endDate *time.Time, timeRange string, client apiv1.API) (model.Matrix, error) {
	queryRange := apiv1.Range{
		Start: *startDate,
		End:   *endDate,
		Step:  time.Minute * 5,
	}

	rangedQuery := strings.Replace(metricQuery, "{{DURATION}}", timeRange, 2)

	value, warnings, err := client.QueryRange(ctx, rangedQuery, queryRange)
	if err != nil {
		return nil, err
	}

	if len(warnings) != 0 {
		fmt.Println(warnings)
	}

	samples, ok := value.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected prometheus query to return a model.Matrix, got %T", value)
	}

	return samples, nil
}

func getTargetSLA(ctx context.Context, slothID string, client apiv1.API, endDate *time.Time) (float64, error) {
	query := strings.Replace(slaQuery, "{{SLOTHID}}", slothID, 1)

	res, warns, err := client.Query(ctx, query, *endDate)
	if err != nil {
		return 0, err
	}

	if len(warns) != 0 {
		fmt.Println(warns)
	}

	samples, ok := res.(model.Vector)
	if !ok {
		return 0, fmt.Errorf("expected prometheus query to return a model.Vector, got %T", res)
	}

	if len(samples) == 0 {
		return 0, errors.New("no target SLA found in prometheus")
	}

	return float64(samples[len(samples)-1].Value) * 100, nil
}
