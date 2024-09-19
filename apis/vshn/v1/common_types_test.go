package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func Test_GetTime(t *testing.T) {
	tests := []struct {
		name         string
		scheduleSpec VSHNDBaaSMaintenanceScheduleSpec
		want         string
	}{
		{
			name: "GivenNormalSchedule_ThenExpectCorrectTimeOfDay",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{
				TimeOfDay: TimeOfDay("00:30:00"),
			},
			want: "00:30:00",
		},
		{
			name: "GivenEmptyTimeOfDay_ThenExpectZeroTimeOfDay",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{
				TimeOfDay: TimeOfDay(""),
			},
			want: "00:00:00",
		},
		{
			name:         "GivenNoTimeOfDay_ThenExpectZeroTimeOfDay",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{},
			want:         "00:00:00",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualTime := tt.scheduleSpec.TimeOfDay.GetTime()
			assert.Equal(t, tt.want, actualTime.Format(time.TimeOnly))
		})
	}
}

func Test_AddTime(t *testing.T) {
	tests := []struct {
		name         string
		scheduleSpec VSHNDBaaSMaintenanceScheduleSpec
		d            time.Duration
		want         string
	}{
		{
			name: "GivenNormalSchedule_ThenExpectAddedTimeOfDay",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{
				TimeOfDay: TimeOfDay("00:30:00"),
			},
			d:    60 * time.Minute,
			want: "01:30:00",
		},
		{
			name: "GivenEmptyTimeOfDay_ThenExpectAddedTimeOfDay",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{
				TimeOfDay: TimeOfDay(""),
			},
			d:    10 * time.Minute,
			want: "00:10:00",
		},
		{
			name:         "GivenNoTimeOfDay_ThenExpectAddedTimeOfDay",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{},
			d:            5 * time.Minute,
			want:         "00:05:00",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualTime := tt.scheduleSpec.TimeOfDay.AddTime(tt.d)
			assert.Equal(t, tt.want, string(actualTime))
		})
	}
}

func Test_IsSet(t *testing.T) {
	tests := []struct {
		name      string
		timeOfDay *TimeOfDay
		want      bool
	}{
		{
			name:      "GivenNormalSchedule_ThenExpectIsTrue",
			timeOfDay: ptr.To[TimeOfDay]("00:00:00"),
			want:      true,
		},
		{
			name:      "GivenEmptyTimeOfDay_ThenExpectIsFalse",
			timeOfDay: ptr.To[TimeOfDay](""),
			want:      false,
		},
		{
			name:      "GivenEmptyTimeOfDay_ThenExpectIsFalse",
			timeOfDay: nil,
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.timeOfDay.IsSet())
		})
	}
}
