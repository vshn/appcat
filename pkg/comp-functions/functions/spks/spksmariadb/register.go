package spksmariadb

import (
	"github.com/vshn/appcat/v4/apis/syntools/v1alpha1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/spks/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService("mariadb-k8s", runtime.Service[*v1alpha1.CompositeMariaDBInstance]{
		Steps: []runtime.Step[*v1alpha1.CompositeMariaDBInstance]{
			{
				Name:    "HandleTLS",
				Execute: HandleTLS,
			},
			// This one should be the last step to call!
			{
				Name:    "fixConnectionDetailsSecret",
				Execute: common.IgnoreConnectionDetailFix[*v1alpha1.CompositeMariaDBInstance],
			},
		},
	})
}
