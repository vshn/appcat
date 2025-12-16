package runtime

import (
	"fmt"
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xpapi "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceRuntime_unwrapUsage(t *testing.T) {
	tests := []struct {
		name    string
		resName string
		objects []client.Object
		want    bool
		wantErr bool
		desired bool
	}{
		{
			name:    "GivenWrappedUsage_ThenUnwrap",
			resName: "myusage",
			objects: []client.Object{
				&xpapi.Usage{},
				&xkube.Object{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myusage",
					},
					Status: xkube.ObjectStatus{
						AtProvider: xkube.ObjectObservation{
							Manifest: runtime.RawExtension{
								Object: &xpapi.Usage{},
							},
						},
					},
				},
			},
			want:    false,
			desired: true,
		},
		{
			name:    "GivenWrappedUsageWithManagementPolicy_ThenUnwrap",
			resName: "myusage",
			objects: []client.Object{
				&xpapi.Usage{},
				&xkube.Object{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myusage",
					},
					Spec: xkube.ObjectSpec{
						ResourceSpec: xpv1.ResourceSpec{
							ManagementPolicies: xpv1.ManagementPolicies{
								xpv1.ManagementActionObserve,
							},
						},
					},
					Status: xkube.ObjectStatus{
						AtProvider: xkube.ObjectObservation{
							Manifest: runtime.RawExtension{
								Object: &xpapi.Usage{},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name:    "GivenUsage_ThenNoUnwrap",
			resName: "myusage",
			objects: []client.Object{
				&xpapi.Usage{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myusage",
					},
					Spec: xpapi.UsageSpec{
						Of: xpapi.Resource{
							Kind: "mykind",
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// We just use something to stand in as a composite...
			// because the runtime reads the name of the composite at one point
			fakeComp := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mycomp",
				},
			}

			s := getTestRuntime(t, tt.objects, fakeComp)

			res, err := s.unwrapUsage(tt.resName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, res)

			if tt.desired {
				assert.NotEmpty(t, s.GetAllDesired())
			} else {
				assert.Empty(t, s.GetAllDesired())
			}

		})
	}
}

func TestServiceRuntime_IsResourceReady(t *testing.T) {
	tests := []struct {
		name      string
		resName   string
		found     bool
		expectErr bool
	}{
		{
			name:    "GivenReadyAndNoSpecialchar_ThenTrue",
			resName: "nospecialchar",
			found:   true,
		},
		{
			name:      "GivenUnReadyAndNoSpecialchar_ThenFalse",
			resName:   "non-ready",
			found:     false,
			expectErr: true,
		},
		{
			name:    "GivenNotSynced_ThenTrue",
			resName: "not-synced",
			found:   true,
		},
		{
			name:      "GivenNotExisting_ThenFalse",
			resName:   "notfound",
			found:     false,
			expectErr: true,
		},
		{
			name:    "GivenUnderline_ThenTrue",
			resName: "no_underline",
			found:   true,
		},
		{
			name:    "GivenCapitalized_ThenTrue",
			resName: "No_Underline",
			found:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// We just use something to stand in as a composite...
			// because the runtime reads the name of the composite at one point
			fakeComp := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mycomp",
				},
			}

			objects := []client.Object{
				&xkube.Object{
					ObjectMeta: metav1.ObjectMeta{
						Name: "no-underline",
					},
					Status: xkube.ObjectStatus{
						ResourceStatus: xpv1.ResourceStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									xpv1.Available(),
								},
							},
						},
					},
				},
				&xkube.Object{
					ObjectMeta: metav1.ObjectMeta{
						Name: "nospecialchar",
					},
					Status: xkube.ObjectStatus{
						ResourceStatus: xpv1.ResourceStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									xpv1.Available(),
								},
							},
						},
					},
				},
				&xkube.Object{
					ObjectMeta: metav1.ObjectMeta{
						Name: "non-ready",
					},
					Status: xkube.ObjectStatus{
						ResourceStatus: xpv1.ResourceStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									xpv1.Unavailable(),
								},
							},
						},
					},
				},
				&xkube.Object{
					ObjectMeta: metav1.ObjectMeta{
						Name: "not-synced",
					},
					Status: xkube.ObjectStatus{
						ResourceStatus: xpv1.ResourceStatus{
							ConditionedStatus: xpv1.ConditionedStatus{
								Conditions: []xpv1.Condition{
									xpv1.Available(),
									xpv1.ReconcileError(fmt.Errorf("oops")),
								},
							},
						},
					},
				},
			}

			s := getTestRuntime(t, objects, fakeComp)

			found, err := s.IsResourceReady(tt.resName)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.found, found)

		})
	}
}

func getTestRuntime(t *testing.T, initialObjects []client.Object, comp client.Object) *ServiceRuntime {
	req := &xfnproto.RunFunctionRequest{
		Observed: &xfnproto.State{
			Resources: map[string]*xfnproto.Resource{},
		},
	}

	for _, obj := range initialObjects {
		res, err := composed.From(obj)
		assert.NoError(t, err)

		protoRes, err := structpb.NewStruct(res.UnstructuredContent())
		assert.NoError(t, err)

		req.Observed.Resources[obj.GetName()] = &xfnproto.Resource{Resource: protoRes}
	}

	compRes, err := composed.From(comp)
	assert.NoError(t, err)

	return &ServiceRuntime{
		req:               req,
		desiredResources:  map[resource.Name]*resource.DesiredComposed{},
		observedComposite: &composite.Unstructured{Unstructured: compRes.Unstructured},
	}
}
