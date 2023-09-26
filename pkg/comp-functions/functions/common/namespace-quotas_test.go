package common

import (
	"context"
	"github.com/vshn/appcat/v4/apis/metadata"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddInitialNamespaceQuotas(t *testing.T) {
	iof := commontest.LoadRuntimeFromFile(t, "common/quotas/01_default.yaml")
	ctx := context.TODO()

	obj := &metadata.MetadataOnlyObject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}

	assert.NoError(t, iof.Desired.SetComposite(ctx, obj))

	res := AddInitialNamespaceQuotas("namespace")(ctx, iof)
	assert.Equal(t, runtime.NewNormal(), res)

	ns := &corev1.Namespace{}
	assert.NoError(t, iof.Desired.GetFromObject(ctx, ns, "namespace"))
	assert.NotEmpty(t, ns.GetAnnotations())
}
