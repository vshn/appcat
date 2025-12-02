package vshnnextcloud

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
)

func Test_addCollabora(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")

	ctx := context.TODO()

	comp.Spec.Parameters.Service.Collabora.Enabled = true
	comp.Spec.Parameters.Service.Collabora.FQDN = "collabora.example.com"

	assert.Nil(t, DeployNextcloud(ctx, comp, svc))

	res := DeployCollabora(ctx, comp, svc)

	assert.True(t, res.Severity == v1.Severity_SEVERITY_NORMAL)

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
		var check func(string) error = func(val string) error {
			if resources[val] != "1" {
				return fmt.Errorf("%s does not exist", val)
			}
			return nil
		}

		assert.NoError(t, check(val))
	}

	comp.Spec.Parameters.Service.Collabora.Enabled = false
	res = DeployCollabora(ctx, comp, svc)

	assert.True(t, res.Severity == v1.Severity_SEVERITY_NORMAL && res.Message == "Collabora not enabled")
}

func Test_addCollaboraDefaultVersion(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")
	ctx := context.TODO()

	// Configure Collabora with default version
	comp.Spec.Parameters.Service.Collabora.Enabled = true
	comp.Spec.Parameters.Service.Collabora.FQDN = "collabora.example.com"

	assert.Nil(t, DeployNextcloud(ctx, comp, svc))

	result := DeployCollabora(ctx, comp, svc)
	assert.True(t, result.Severity == v1.Severity_SEVERITY_NORMAL)

	collaboraBaseImage := svc.Config.Data["collabora_image"]
	defaultImageTag := svc.Config.Data["collabora_image_tag"]
	expectedImage := fmt.Sprintf("%s:%s", collaboraBaseImage, defaultImageTag)

	sts := &appsv1.StatefulSet{}
	assert.NoError(t, svc.GetDesiredKubeObject(sts, comp.GetName()+"-collabora-code-sts"))
	assert.Equal(t, expectedImage, sts.Spec.Template.Spec.Containers[0].Image)
}

func Test_addCollaboraCustomVersion(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")
	ctx := context.TODO()

	// Configure Collabora with custom version
	customVersion := "25.04.1.1.1"
	comp.Spec.Parameters.Service.Collabora.Enabled = true
	comp.Spec.Parameters.Service.Collabora.FQDN = "collabora.example.com"
	comp.Spec.Parameters.Service.Collabora.Version = customVersion

	assert.Nil(t, DeployNextcloud(ctx, comp, svc))

	result := DeployCollabora(ctx, comp, svc)
	assert.True(t, result.Severity == v1.Severity_SEVERITY_NORMAL)

	collaboraBaseImage := svc.Config.Data["collabora_image"]
	expectedImage := fmt.Sprintf("%s:%s", collaboraBaseImage, customVersion)

	sts := &appsv1.StatefulSet{}
	assert.NoError(t, svc.GetDesiredKubeObject(sts, comp.GetName()+"-collabora-code-sts"))
	assert.Equal(t, expectedImage, sts.Spec.Template.Spec.Containers[0].Image)
}

func Test_addCollaboraSuspendedInstance(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")
	ctx := context.TODO()

	// Configure Collabora with enabled but instances set to 0
	comp.Spec.Parameters.Service.Collabora.Enabled = true
	comp.Spec.Parameters.Service.Collabora.FQDN = "collabora.example.com"
	comp.Spec.Parameters.Instances = 0

	assert.Nil(t, DeployNextcloud(ctx, comp, svc))

	result := DeployCollabora(ctx, comp, svc)
	assert.True(t, result.Severity == v1.Severity_SEVERITY_NORMAL)

	// Verify that Collabora StatefulSet has 0 replicas when instance is suspended
	sts := &appsv1.StatefulSet{}
	assert.NoError(t, svc.GetDesiredKubeObject(sts, comp.GetName()+"-collabora-code-sts"))
	assert.Equal(t, int32(0), *sts.Spec.Replicas, "Collabora should have 0 replicas when instance is suspended")

	// Now test with instances > 0
	comp.Spec.Parameters.Instances = 2
	assert.Nil(t, DeployNextcloud(ctx, comp, svc))
	result = DeployCollabora(ctx, comp, svc)
	assert.True(t, result.Severity == v1.Severity_SEVERITY_NORMAL)

	assert.NoError(t, svc.GetDesiredKubeObject(sts, comp.GetName()+"-collabora-code-sts"))
	assert.Equal(t, int32(1), *sts.Spec.Replicas, "Collabora should have 1 replica when instance is active")
}
