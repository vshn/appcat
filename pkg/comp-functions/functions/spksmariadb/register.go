package spksmariadb

import (
	"github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("mariadb-k8s", runtime.Service[*v1alpha1.CompositeMariaDBInstance]{
		Steps: []runtime.Step[*v1alpha1.CompositeMariaDBInstance]{
			{
				Name:    "resizePVC",
				Execute: ResizeSpksPVCs,
			},
		},
	})
}
