package runtime

import (
	"testing"

	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetCompositionRevisionSelectorLabel(t *testing.T) {
	xr := composite.New()
	xr.SetCompositionRevisionSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{"metadata.appcat.vshn.io/revision": "v3.72.2-v4.186.2"},
	})
	s := &ServiceRuntime{observedComposite: xr}
	assert.Equal(t, "v3.72.2-v4.186.2", s.GetCompositionRevisionSelectorLabel("metadata.appcat.vshn.io/revision"))
}

func TestGetCompositionRevisionSelectorLabel_noSelector(t *testing.T) {
	s := &ServiceRuntime{observedComposite: composite.New()}
	assert.Equal(t, "", s.GetCompositionRevisionSelectorLabel("metadata.appcat.vshn.io/revision"))
}
