package vshnpostgres

import (
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	"k8s.io/utils/ptr"
	"testing"
)

func Test_IsSuccessful(t *testing.T) {
	tests := []struct {
		name       string
		conditions *[]sgv1.SGDbOpsStatusConditionsItem
		expected   bool
	}{
		{
			name:       "Nil conditions",
			conditions: nil,
			expected:   false,
		},
		{
			name:       "Empty conditions",
			conditions: &[]sgv1.SGDbOpsStatusConditionsItem{},
			expected:   false,
		},
		{
			name: "OperationCompleted is True, no OperationFailed",
			conditions: &[]sgv1.SGDbOpsStatusConditionsItem{
				{Reason: ptr.To("OperationCompleted"), Status: ptr.To("True")},
			},
			expected: true,
		},
		{
			name: "OperationFailed is True",
			conditions: &[]sgv1.SGDbOpsStatusConditionsItem{
				{Reason: ptr.To("OperationFailed"), Status: ptr.To("True")},
			},
			expected: false,
		},
		{
			name: "Both OperationCompleted and OperationFailed are True",
			conditions: &[]sgv1.SGDbOpsStatusConditionsItem{
				{Reason: ptr.To("OperationCompleted"), Status: ptr.To("True")},
				{Reason: ptr.To("OperationFailed"), Status: ptr.To("True")},
			},
			expected: false,
		},
		{
			name: "OperationRunning is True",
			conditions: &[]sgv1.SGDbOpsStatusConditionsItem{
				{Reason: ptr.To("OperationRunning"), Status: ptr.To("True")},
			},
			expected: false,
		},
		{
			name: "OperationCompleted is True, other conditions exist",
			conditions: &[]sgv1.SGDbOpsStatusConditionsItem{
				{Reason: ptr.To("OperationCompleted"), Status: ptr.To("True")},
				{Reason: ptr.To("OperationNotRunning"), Status: ptr.To("True")},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSuccessful(tt.conditions)
			if result != tt.expected {
				t.Errorf("isSuccessful() = %v, want %v", result, tt.expected)
			}
		})
	}
}
