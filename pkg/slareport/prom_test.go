package slareport

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func Test_parseRange(t *testing.T) {
	type args struct {
		date     string
		duration string
	}
	tests := []struct {
		name      string
		args      args
		startDate time.Time
		endDate   time.Time
		wantErr   bool
	}{
		{
			name: "GivenValidDateAndDuration_ThenExpectOutput",
			args: args{
				date:     "2023-06-07T09:00:00Z",
				duration: "30d",
			},
			endDate:   time.Date(2023, 6, 7, 9, 0, 0, 0, time.UTC),
			startDate: time.Date(2023, 5, 8, 9, 0, 0, 0, time.UTC),
		},
		{
			name: "GivenInvalidDate_ThenExpectError",
			args: args{
				date:     "foo",
				duration: "30d",
			},
			wantErr: true,
		},
		{
			name: "GivenInvalidDuration_ThenExpectError",
			args: args{
				date:     "2023-06-07T09:00:00Z",
				duration: "30tage",
			},
			wantErr: true,
		},
		{
			name: "GivenNonDefaultDuration_ThenExpectOutput",
			args: args{
				date:     "2023-06-07T09:00:00Z",
				duration: "5m",
			},
			endDate:   time.Date(2023, 6, 7, 9, 0, 0, 0, time.UTC),
			startDate: time.Date(2023, 6, 7, 8, 55, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startDate, endDate, err := parseRange(tt.args.date, tt.args.duration)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, startDate, &tt.startDate)
				assert.Equal(t, endDate, &tt.endDate)
			}
		})
	}
}

func TestRunQuery(t *testing.T) {
	tests := []struct {
		name    string
		metrics model.Matrix
		want    map[string][]ServiceInstance
		wantErr bool
	}{
		{
			name: "GivenAvailableMetrics_ThenExpectOutput",
			want: map[string][]ServiceInstance{
				"mycustomer": {
					{
						Namespace:  "myns",
						Instance:   "test",
						TargetSLA:  99.9,
						OutcomeSLA: 99.99,
						Cluster:    "mycluster",
						Service:    "postgresql",
						Color:      "green",
					},
				},
			},
			metrics: model.Matrix{
				{
					Metric: model.Metric{
						"name":                         "test",
						"namespace":                    "myns",
						"service":                      "postgresql",
						"cluster_id":                   "mycluster",
						"label_appuio_io_organization": "mycustomer",
					},
					Values: []model.SamplePair{
						{
							Timestamp: model.Time(time.Now().UnixMilli()),
							Value:     model.SampleValue(99.99),
						},
						{
							Timestamp: model.Time(time.Now().UnixMilli()),
							Value:     model.SampleValue(99.99),
						},
					},
				},
			},
		},
		{
			name: "GivenUnreachedSLA_ThenExpectColorRed",
			want: map[string][]ServiceInstance{
				"mycustomer": {
					{
						Namespace:  "myns",
						Instance:   "test",
						TargetSLA:  99.9,
						OutcomeSLA: 99.8,
						Cluster:    "mycluster",
						Service:    "postgresql",
						Color:      "red",
					},
				},
			},
			metrics: model.Matrix{
				{
					Metric: model.Metric{
						"name":                         "test",
						"namespace":                    "myns",
						"service":                      "postgresql",
						"cluster_id":                   "mycluster",
						"label_appuio_io_organization": "mycustomer",
					},
					Values: []model.SamplePair{
						{
							Timestamp: model.Time(time.Now().UnixMilli()),
							Value:     model.SampleValue(99.8),
						},
						{
							Timestamp: model.Time(time.Now().UnixMilli()),
							Value:     model.SampleValue(99.8),
						},
					},
				},
			},
		},
		{
			name:    "GivenNoMetrics_ThenExpectEmptyOutput",
			want:    map[string][]ServiceInstance{},
			metrics: model.Matrix{},
		},
		{
			name: "GivenNoValues_ThenExpectAnError",
			want: map[string][]ServiceInstance{},
			metrics: model.Matrix{
				{
					Metric: model.Metric{
						"name":                         "test",
						"namespace":                    "myns",
						"service":                      "postgresql",
						"cluster_id":                   "mycluster",
						"label_appuio_io_organization": "mycustomer",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {

		promClientFunc = getDummyPromClient
		getTargetSLAFunc = getDummySLA
		getMetricsFunc = func(ctx context.Context, startDate, endDate *time.Time, timeRange string, client apiv1.API) (model.Matrix, error) {
			return tt.metrics, nil
		}

		t.Run(tt.name, func(t *testing.T) {
			got, err := RunQuery(context.TODO(), "dummy", "30d", "2023-06-07T09:00:00Z")
			if tt.wantErr {
				assert.Error(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func getDummyPromClient(promURL string, thanosAllowPartialResponses bool, orgID string) (apiv1.API, error) {
	client, _ := api.NewClient(api.Config{})

	return apiv1.NewAPI(client), nil
}

func getDummySLA(ctx context.Context, slothID string, client apiv1.API, endDate *time.Time) (float64, error) {
	return 99.9, nil
}
