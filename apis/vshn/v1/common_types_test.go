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

func Test_AddDuration_BasicCases(t *testing.T) {
	tests := []struct {
		name      string
		timeOfDay TimeOfDay
		d         time.Duration
		want      string
	}{
		{
			name:      "GivenNormalSchedule_ThenExpectAddedTimeOfDay",
			timeOfDay: TimeOfDay("00:30:00"),
			d:         60 * time.Minute,
			want:      "01:30:00",
		},
		{
			name:      "GivenEmptyTimeOfDay_ThenExpectAddedTimeOfDay",
			timeOfDay: TimeOfDay(""),
			d:         10 * time.Minute,
			want:      "00:10:00",
		},
		{
			name:      "GivenNoTimeOfDay_ThenExpectAddedTimeOfDay",
			timeOfDay: TimeOfDay(""),
			d:         5 * time.Minute,
			want:      "00:05:00",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultTime, _ := tt.timeOfDay.AddDuration(tt.d)
			assert.Equal(t, tt.want, string(resultTime))
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

func Test_PinImageTag(t *testing.T) {
	tests := []struct {
		name         string
		scheduleSpec VSHNDBaaSMaintenanceScheduleSpec
		wantTag      string
		wantIsSet    bool
	}{
		{
			name: "GivenPinImageTagSet_ThenExpectTagAndTrue",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{
				PinImageTag: "7.2.5",
			},
			wantTag:   "7.2.5",
			wantIsSet: true,
		},
		{
			name:         "GivenDefaultValue_ThenExpectEmptyAndFalse",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{},
			wantTag:      "",
			wantIsSet:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantTag, tt.scheduleSpec.GetPinImageTag())
			assert.Equal(t, tt.wantIsSet, tt.scheduleSpec.IsPinImageTagSet())
		})
	}
}

func Test_IsAppcatReleaseDisabled(t *testing.T) {
	tests := []struct {
		name         string
		scheduleSpec VSHNDBaaSMaintenanceScheduleSpec
		want         bool
	}{
		{
			name: "GivenDisableAppcatReleaseTrue_ThenExpectTrue",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{
				DisableAppcatRelease: true,
			},
			want: true,
		},
		{
			name:         "GivenDefaultValue_ThenExpectFalse",
			scheduleSpec: VSHNDBaaSMaintenanceScheduleSpec{},
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.scheduleSpec.IsAppcatReleaseDisabled())
		})
	}
}

func Test_AddDuration(t *testing.T) {
	tests := []struct {
		name          string
		timeOfDay     TimeOfDay
		duration      time.Duration
		wantTime      string
		wantDayOffset int
	}{
		{
			name:          "GivenMidnightRollover_ThenExpectNextDay",
			timeOfDay:     TimeOfDay("23:50:00"),
			duration:      20 * time.Minute,
			wantTime:      "00:10:00",
			wantDayOffset: 1,
		},
		{
			name:          "GivenSameDay_ThenExpectNoDayOffset",
			timeOfDay:     TimeOfDay("10:00:00"),
			duration:      30 * time.Minute,
			wantTime:      "10:30:00",
			wantDayOffset: 0,
		},
		{
			name:          "GivenRolloverToMidnight_ThenExpectNextDay",
			timeOfDay:     TimeOfDay("23:00:00"),
			duration:      1 * time.Hour,
			wantTime:      "00:00:00",
			wantDayOffset: 1,
		},
		{
			name:          "GivenNegativeDuration_ThenExpectPreviousDay",
			timeOfDay:     TimeOfDay("00:10:00"),
			duration:      -20 * time.Minute,
			wantTime:      "23:50:00",
			wantDayOffset: -1,
		},
		{
			name:          "GivenLargePositiveDuration_ThenExpectPositiveDayOffset",
			timeOfDay:     TimeOfDay("10:00:00"),
			duration:      15 * time.Hour,
			wantTime:      "01:00:00",
			wantDayOffset: 1,
		},
		{
			name:          "GivenExactly24Hours_ThenExpectOneDayOffset",
			timeOfDay:     TimeOfDay("10:00:00"),
			duration:      24 * time.Hour,
			wantTime:      "10:00:00",
			wantDayOffset: 1,
		},
		{
			name:          "Given25HoursWithRollover_ThenExpectTwoDayOffset",
			timeOfDay:     TimeOfDay("23:00:00"),
			duration:      25 * time.Hour,
			wantTime:      "00:00:00",
			wantDayOffset: 2,
		},
		{
			name:          "Given36HoursWithRollover_ThenExpectTwoDayOffset",
			timeOfDay:     TimeOfDay("12:00:00"),
			duration:      36 * time.Hour,
			wantTime:      "00:00:00",
			wantDayOffset: 2,
		},
		{
			name:          "Given50HoursWithRollover_ThenExpectThreeDayOffset",
			timeOfDay:     TimeOfDay("22:00:00"),
			duration:      50 * time.Hour,
			wantTime:      "00:00:00",
			wantDayOffset: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultTime, dayOffset := tt.timeOfDay.AddDuration(tt.duration)
			assert.Equal(t, tt.wantTime, string(resultTime), "Result time should match expected")
			assert.Equal(t, tt.wantDayOffset, dayOffset, "Day offset should match expected")
		})
	}
}

func Test_AddDaysToWeekday(t *testing.T) {
	tests := []struct {
		name    string
		weekday string
		days    int
		want    string
	}{
		{
			name:    "GivenMondayPlus1_ThenExpectTuesday",
			weekday: "monday",
			days:    1,
			want:    "tuesday",
		},
		{
			name:    "GivenMondayPlus0_ThenExpectMonday",
			weekday: "monday",
			days:    0,
			want:    "monday",
		},
		{
			name:    "GivenSundayPlus1_ThenExpectMonday",
			weekday: "sunday",
			days:    1,
			want:    "monday",
		},
		{
			name:    "GivenMondayPlus7_ThenExpectMonday",
			weekday: "monday",
			days:    7,
			want:    "monday",
		},
		{
			name:    "GivenMondayMinus1_ThenExpectSunday",
			weekday: "monday",
			days:    -1,
			want:    "sunday",
		},
		{
			name:    "GivenSaturdayPlus2_ThenExpectMonday",
			weekday: "saturday",
			days:    2,
			want:    "monday",
		},
		{
			name:    "GivenInvalidWeekday_ThenExpectUnchanged",
			weekday: "invalid",
			days:    1,
			want:    "invalid",
		},
		{
			name:    "GivenWednesdayPlus10_ThenExpectSaturday",
			weekday: "wednesday",
			days:    10,
			want:    "saturday",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddDaysToWeekday(tt.weekday, tt.days)
			assert.Equal(t, tt.want, result)
		})
	}
}
