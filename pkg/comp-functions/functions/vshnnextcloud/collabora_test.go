package vshnnextcloud

import (
	"context"
	"strings"
	"testing"

	"github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/stretchr/testify/assert"
)

func Test_addCollabora(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")

	ctx := context.TODO()

	comp.Spec.Parameters.Service.Collabora.Enabled = true
	comp.Spec.Parameters.Service.Collabora.FQDN = "collabora.example.com"

	assert.Nil(t, DeployNextcloud(ctx, comp, svc))

	res := DeployCollabora(ctx, comp, svc)

	assert.True(t, res.Severity == v1beta1.Severity_SEVERITY_NORMAL)

	// collabora function returned coorect result, now checking for existence of all required resources

	collabora_objects := []string{
		comp.GetName() + "-collabora-code-coolwsd-config",
		comp.GetName() + "-collabora-code-sts",
		comp.GetName() + "-collabora-code-rolebinding",
		comp.GetName() + "-collabora-code-ingress",
		comp.GetName() + "-collabora-code-wo-secret",
		comp.GetName() + "-collabora-code-serviceaccount",
		comp.GetName() + "-collabora-code-certificate",
		comp.GetName() + "-collabora-code-role",
		comp.GetName() + "-collabora-code-issuer",
		comp.GetName() + "-collabora-code-service",
	}

	resources := map[string]string{}

	x := svc.GetAllDesired()

	for _, val := range x {
		if strings.Contains(val.Resource.GetName(), "collabora-code") {
			resources[val.Resource.GetName()] = "1"
		}
	}

	for _, val := range collabora_objects {
		assert.Equal(t, "1", resources[val])
	}

	comp.Spec.Parameters.Service.Collabora.Enabled = false
	res = DeployCollabora(ctx, comp, svc)

	assert.True(t, res.Severity == v1beta1.Severity_SEVERITY_NORMAL && res.Message == "Collabora not enabled")

	comp.Spec.Parameters.Service.Collabora.Enabled = true
	comp.Spec.Parameters.Service.Collabora.FQDN = ""
	res = DeployCollabora(ctx, comp, svc)

	assert.True(t, res.Severity == v1beta1.Severity_SEVERITY_WARNING && res.Message == "Collabora FQDN is not a valid DNS name: "+comp.Spec.Parameters.Service.Collabora.FQDN)

}
