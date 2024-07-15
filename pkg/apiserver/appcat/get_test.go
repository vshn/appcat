package appcat

import (
	"testing"

	crossplanev1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestAppCatStorage_Get(t *testing.T) {
	tests := map[string]struct {
		name        string
		composition *crossplanev1.Composition
		compErr     error
		appcat      *v1.AppCat
		err         error
	}{
		"GivenAComposition_ThenAppCat": {
			name:        "one",
			composition: compositionOne,
			appcat:      appCatOne,
		},
		"GivenErrNotFound_ThenErrNotFound": {
			name: "not-found",
			compErr: apierrors.NewNotFound(schema.GroupResource{
				Resource: "compositions",
			}, "not-found"),
			err: apierrors.NewNotFound(schema.GroupResource{
				Group:    v1.GroupVersion.Group,
				Resource: "appcats",
			}, "not-found"),
		},
		"GivenNonAppCatComp_ThenErrNotFound": {
			name: "appcat-not-found",
			composition: &crossplanev1.Composition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1.OfferedKey: "false",
					},
				},
			},
			err: apierrors.NewNotFound(schema.GroupResource{
				Group:    v1.GroupVersion.Group,
				Resource: "appcats",
			}, "appcat-not-found"),
		},
	}

	for n, tc := range tests {

		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			stor, compProvider := newMockedAppCatStorage(t, ctrl)

			compProvider.EXPECT().
				GetComposition(gomock.Any(), tc.name, gomock.Any()).
				Return(tc.composition, tc.compErr).
				Times(1)

			appcat, err := stor.Get(request.WithRequestInfo(request.NewContext(),
				&request.RequestInfo{
					Verb:     "get",
					APIGroup: v1.GroupVersion.Group,
					Resource: "appcats",
					Name:     tc.name,
				}),
				tc.name, nil)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.appcat, appcat)
		})
	}
}
