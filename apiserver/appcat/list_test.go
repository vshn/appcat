package appcat

import (
	crossplanev1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apiserver/pkg/endpoints/request"
)

func TestAppcatStorage_List(t *testing.T) {
	tests := map[string]struct {
		compositions   *crossplanev1.CompositionList
		compositionErr error

		appcats *v1.AppCatList
		err     error
	}{
		"GivenListOfCompositions_ThenReturnAppCats": {
			compositions: &crossplanev1.CompositionList{
				Items: []crossplanev1.Composition{
					*compositionOne,
					*compositionTwo,
				},
			},
			appcats: &v1.AppCatList{
				Items: []v1.AppCat{
					*appCatOne,
					*appCatTwo,
				},
			},
		},
		"GivenErrNotFound_ThenErrNotFound": {
			compositionErr: apierrors.NewNotFound(schema.GroupResource{
				Resource: "compositions",
			}, "not-found"),
			err: apierrors.NewNotFound(schema.GroupResource{
				Group:    v1.GroupVersion.Group,
				Resource: v1.Resource,
			}, "not-found"),
		},
		"GivenList_ThenFilter": {
			compositions: &crossplanev1.CompositionList{
				Items: []crossplanev1.Composition{
					*compositionOne,
					*compositionNonOffered,
				},
			},
			appcats: &v1.AppCatList{
				Items: []v1.AppCat{
					*appCatOne,
				},
			},
		},
	}
	for n, tc := range tests {
		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			stor, compProvider := newMockedAppCatStorage(t, ctrl)

			compProvider.EXPECT().
				ListCompositions(gomock.Any(), gomock.Any()).
				Return(tc.compositions, tc.compositionErr).
				Times(1)

			appcats, err := stor.List(request.WithRequestInfo(request.NewContext(),
				&request.RequestInfo{
					Verb:     "list",
					APIGroup: v1.GroupVersion.Group,
					Resource: v1.Resource,
				}), nil)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.appcats, appcats)
		})
	}
}

type testWatcher struct {
	events chan watch.Event
}

func (w testWatcher) Stop() {}

func (w testWatcher) ResultChan() <-chan watch.Event {
	return w.events
}

func TestAppCatsStorage_Watch(t *testing.T) {
	tests := map[string]struct {
		compositionEvents []watch.Event
		compositionErr    error

		appcatEvents []watch.Event
		err          error
	}{
		"GivenCompositionEvents_ThenAppCatEvents": {
			compositionEvents: []watch.Event{
				{
					Type:   watch.Added,
					Object: compositionOne,
				},
				{
					Type:   watch.Modified,
					Object: compositionTwo,
				},
			},
			appcatEvents: []watch.Event{
				{
					Type:   watch.Added,
					Object: appCatOne,
				},
				{
					Type:   watch.Modified,
					Object: appCatTwo,
				},
			},
		},
		"GivenErrNotFound_ThenErrNotFound": {
			compositionErr: apierrors.NewNotFound(schema.GroupResource{
				Resource: "compositions",
			}, "not-found"),
			err: apierrors.NewNotFound(schema.GroupResource{
				Group:    v1.GroupVersion.Group,
				Resource: v1.Resource,
			}, "not-found"),
		},
		"GivenVariousCompositionEvents_ThenFilter": {
			compositionEvents: []watch.Event{
				{
					Type:   watch.Added,
					Object: compositionOne,
				},
				{
					Type:   watch.Modified,
					Object: compositionNonOffered,
				},
				{
					Type:   watch.Modified,
					Object: compositionTwo,
				},
			},
			appcatEvents: []watch.Event{
				{
					Type:   watch.Added,
					Object: appCatOne,
				},
				{
					Type:   watch.Modified,
					Object: appCatTwo,
				},
			},
		},
	}

	for n, tc := range tests {
		t.Run(n, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			stor, compProvider := newMockedAppCatStorage(t, ctrl)

			compWatcher := testWatcher{
				events: make(chan watch.Event, len(tc.compositionEvents)),
			}
			for _, e := range tc.compositionEvents {
				compWatcher.events <- e
			}
			close(compWatcher.events)

			compProvider.EXPECT().
				WatchCompositions(gomock.Any(), gomock.Any()).
				Return(compWatcher, tc.compositionErr).
				AnyTimes()

			appcatWatch, err := stor.Watch(request.WithRequestInfo(request.NewContext(),
				&request.RequestInfo{
					Verb:     "watch",
					APIGroup: v1.GroupVersion.Group,
					Resource: v1.Resource,
				}), nil)
			if tc.err != nil {
				assert.EqualError(t, err, tc.err.Error())
				return
			}
			require.NoError(t, err)
			appcatEvents := []watch.Event{}
			for e := range appcatWatch.ResultChan() {
				appcatEvents = append(appcatEvents, e)
			}
			assert.Equal(t, tc.appcatEvents, appcatEvents)
		})
	}
}
