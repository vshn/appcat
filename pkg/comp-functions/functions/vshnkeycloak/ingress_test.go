package vshnkeycloak

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
)

func TestEnableIngresValues(t *testing.T) {
	type args struct {
		comp   *vshnv1.VSHNKeycloak
		values map[string]any
	}

	tests := []struct {
		name string
		args args
		want map[string]any
	}{
		{
			name: "GivenFQDN_Then_ExpectIngress",
			args: struct {
				comp   *vshnv1.VSHNKeycloak
				values map[string]any
			}{
				comp: &vshnv1.VSHNKeycloak{
					Spec: vshnv1.VSHNKeycloakSpec{
						Parameters: vshnv1.VSHNKeycloakParameters{
							Service: vshnv1.VSHNKeycloakServiceSpec{
								FQDN:         "example.com",
								RelativePath: "/path",
							},
						},
					},
				},
				values: map[string]any{},
			},
			want: map[string]any{
				"ingress": map[string]any{
					"enabled":     true,
					"servicePort": "https",
					"annotations": map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
					},
					"rules": []map[string]any{
						{
							"host": "example.com",
							"paths": []map[string]any{
								{
									"path":     `'{{ tpl .Values.http.relativePath $ | trimSuffix " / " }}/'`,
									"pathType": "Prefix",
								},
							},
						},
					},
					"tls": []map[string]any{
						{
							"hosts": []string{"example.com"},
						},
					},
				},
			},
		},
		{
			name: "GivenNoFQDN_Then_ExpectNoIngress",
			args: struct {
				comp   *vshnv1.VSHNKeycloak
				values map[string]any
			}{
				comp: &vshnv1.VSHNKeycloak{
					Spec: vshnv1.VSHNKeycloakSpec{},
				},
				values: map[string]any{},
			},
			want: map[string]any{
				"ingress": map[string]any{
					"enabled":     true,
					"servicePort": "https",
					"annotations": map[string]string{
						"nginx.ingress.kubernetes.io/backend-protocol": "HTTPS",
					},
					"rules": []map[string]any{
						{
							"host": "",
							"paths": []map[string]any{
								{
									"path":     `'{{ tpl .Values.http.relativePath $ | trimSuffix " / " }}/'`,
									"pathType": "Prefix",
								},
							},
						},
					},
					"tls": []map[string]any{
						{
							"hosts": []string{""},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enableIngresValues(tt.args.comp, tt.args.values)
			assert.Equal(t, tt.want, tt.args.values)
		})
	}
}

func TestAddIngress(t *testing.T) {
	// Given
	svc := commontest.LoadRuntimeFromFile(t, "vshnkeycloak/02_withFQDN.yaml")

	// When
	res := AddIngress(context.TODO(), svc)
	assert.Nil(t, res)

	// Then
	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, "keycloak-app1-prod-release"))

	values, err := common.GetDesiredReleaseValues(svc, "keycloak-app1-prod-release")
	assert.NoError(t, err)
	assert.NotEmpty(t, values)
	assert.NotEmpty(t, values["ingress"])

}
