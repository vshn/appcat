package appcat

import (
	crossplanev1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1 "github.com/vshn/appcat-apiserver/apis/appcat/v1"
	"github.com/vshn/appcat-apiserver/test/mocks"
	"k8s.io/apiserver/pkg/registry/rest"
	"testing"

	"github.com/golang/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newMockedAppCatStorage is a mocked instance of AppCatStorage
func newMockedAppCatStorage(t *testing.T, ctrl *gomock.Controller) (rest.StandardStorage, *mocks.MockcompositionProvider) {
	t.Helper()
	comp := mocks.NewMockcompositionProvider(ctrl)
	stor := &appcatStorage{
		compositions: comp,
	}
	return rest.Storage(stor).(rest.StandardStorage), comp
}

// Test AppCat instances
var (
	appCatOne = &v1.AppCat{
		ObjectMeta: metav1.ObjectMeta{
			Name: "one",
		},
		Spec: map[string]string{
			"zone":        "rma1",
			"displayname": "one",
			"docs":        "https://docs.com",
		},
		Status: v1.AppCatStatus{
			CompositionName: "one",
		},
	}
	compositionOne = &crossplanev1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "one",
			Labels: map[string]string{
				v1.OfferedKey: v1.OfferedValue,
			},
			Annotations: map[string]string{
				v1.PrefixAppCatKey + "/zone":        "rma1",
				v1.PrefixAppCatKey + "/displayname": "one",
				v1.PrefixAppCatKey + "/docs":        "https://docs.com",
			},
		},
	}
	appCatTwo = &v1.AppCat{
		ObjectMeta: metav1.ObjectMeta{
			Name: "two",
		},
		Spec: map[string]string{
			"zone":               "lpg",
			"displayname":        "two",
			"docs":               "https://docs.com",
			"productDescription": "product desc",
		},
		Status: v1.AppCatStatus{
			CompositionName: "two",
		},
	}
	compositionTwo = &crossplanev1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "two",
			Labels: map[string]string{
				v1.OfferedKey: v1.OfferedValue,
			},
			Annotations: map[string]string{
				v1.PrefixAppCatKey + "/zone":                "lpg",
				v1.PrefixAppCatKey + "/displayname":         "two",
				v1.PrefixAppCatKey + "/docs":                "https://docs.com",
				v1.PrefixAppCatKey + "/product-description": "product desc",
			},
		},
	}
	compositionNonOffered = &crossplanev1.Composition{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				v1.OfferedKey: "false",
			},
		},
	}
)
