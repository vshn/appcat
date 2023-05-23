package v1

import (
	"testing"

	crossplanev1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewAppCatFromComposition(t *testing.T) {
	tests := map[string]struct {
		composition *crossplanev1.Composition
		appCat      *AppCat
	}{
		"GivenNil_ThenNil": {},
		"GivenNoLabels_ThenNil": {
			composition: &crossplanev1.Composition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
			},
		},
		"GivenNonOfferedLabel_ThenNil": {
			composition: &crossplanev1.Composition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						OfferedKey: "false",
					},
				},
			},
		},
		"GivenMissingOfferedLabel_ThenNil": {
			composition: &crossplanev1.Composition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"labelname": "labelvalue",
					},
				},
			},
		},
		"GivenOfferedLabelWithAppCatAnnotations_ThenReturnAppCat": {
			composition: &crossplanev1.Composition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						OfferedKey: OfferedValue,
					},
					Annotations: map[string]string{
						PrefixAppCatKey + "/zone":            "rma1",
						"non-appcat-prefix" + "/displayname": "comp-1",
						"non-appcat-prefix" + "/pippo":       "value-23",
					},
					Name: "comp-1",
				},
			},
			appCat: &AppCat{
				ObjectMeta: metav1.ObjectMeta{
					Name: "comp-1",
				},

				ServiceMetadata: map[string]string{
					"zone": "rma1",
				},

				Status: AppCatStatus{
					CompositionName: "comp-1",
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			actualAppCat := NewAppCatFromComposition(tt.composition)
			assert.DeepEqual(t, tt.appCat, actualAppCat)
		})
	}
}

func TestMakeCamelCase(t *testing.T) {
	tests := map[string]struct {
		input, output string
	}{
		"GivenWrongK8sStrCase1_ThenCamelCaseStr": {
			input:  "-k8s-name-type-",
			output: "k8sNameType",
		},
		"GivenWrongK8sStrCase2_ThenCamelCaseStr": {
			input:  "::-k8s-name-.type/",
			output: "k8sNameType",
		},
		"GivenWrongK8sStrCase3_ThenCamelCaseStr": {
			input:  "-k8S-nA%me-tyPe%",
			output: "k8sNameType",
		},
		"GivenCorrectK8sStr_ThenCamelCaseStr": {
			input:  "k8s-name-type",
			output: "k8sNameType",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			actualStr := makeCamelCase(tt.input)
			assert.Equal(t, tt.output, actualStr)
		})
	}
}
