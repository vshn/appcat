package runtime

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xpapi "github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/stretchr/testify/assert"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	"google.golang.org/protobuf/types/known/structpb"
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

			req := &xfnproto.RunFunctionRequest{
				Observed: &xfnproto.State{
					Resources: map[string]*xfnproto.Resource{},
				},
			}

			for _, obj := range tt.objects {
				res, err := composed.From(obj)
				assert.NoError(t, err)

				protoRes, err := structpb.NewStruct(res.UnstructuredContent())
				assert.NoError(t, err)

				req.Observed.Resources[obj.GetName()] = &xfnproto.Resource{Resource: protoRes}
			}

			s := &ServiceRuntime{
				req:              req,
				desiredResources: map[resource.Name]*resource.DesiredComposed{},
			}

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
