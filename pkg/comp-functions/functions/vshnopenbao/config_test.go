package vshnopenbao

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildHclConfig(t *testing.T) {
	svc, comp := getOpenBaoTestComp(t)
	ctx := context.TODO()

	// Create ConfigMap with HCL
	assert.Nil(t, CreateHCLConfigMap(ctx, comp, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetObservedKubeObject(ns, "openbao-test-ns"))

	// Test HCL config secret creation
	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, "openbao-test-hcl-config"))

	// Verify secret has the expected namespace
	assert.Equal(t, "vshn-openbao-openbao-test", secret.Namespace)

	// Verify secret contains the HCL config data
	require.Contains(t, secret.Data, "config.hcl", "Secret should contain config.hcl key")
	hclBytes := secret.Data["config.hcl"]
	require.NotEmpty(t, hclBytes, "HCL config should not be empty")
}

func TestEncodeDecodeHCL(t *testing.T) {
	// Create a test config
	originalConfig := &OpenBaoConfig{
		UI:          true,
		LogLevel:    "info",
		LogFormat:   "json",
		ClusterName: "test-cluster",
		Listeners: []ListenerBlock{
			{
				Type:           "tcp",
				Address:        "[::]:8200",
				ClusterAddress: "[::]:8201",
				TLSDisable:     false,
				TLSCertFile:    "/certs/tls.crt",
				TLSKeyFile:     "/certs/tls.key",
			},
		},
	}

	// Encode to HCL
	hclBytes := EncodeHCL(originalConfig)
	require.NotEmpty(t, hclBytes, "Encoded HCL should not be empty")

	// Decode back to config
	decodedConfig, err := DecodeHCL(hclBytes, "config.hcl")
	require.NoError(t, err, "Decoding should succeed")
	require.NotNil(t, decodedConfig, "Decoded config should not be nil")

	// Verify the round-trip encoding/decoding preserves values
	assert.Equal(t, originalConfig.UI, decodedConfig.UI)
	assert.Equal(t, originalConfig.LogLevel, decodedConfig.LogLevel)
	assert.Equal(t, originalConfig.LogFormat, decodedConfig.LogFormat)
	assert.Equal(t, originalConfig.ClusterName, decodedConfig.ClusterName)
	assert.Equal(t, originalConfig.APIAddr, decodedConfig.APIAddr)
	assert.Equal(t, originalConfig.ClusterAddr, decodedConfig.ClusterAddr)
	assert.Len(t, decodedConfig.Listeners, len(originalConfig.Listeners))

	// Verify listener details
	if len(originalConfig.Listeners) > 0 && len(decodedConfig.Listeners) > 0 {
		origListener := originalConfig.Listeners[0]
		decListener := decodedConfig.Listeners[0]
		assert.Equal(t, origListener.Type, decListener.Type)
		assert.Equal(t, origListener.Address, decListener.Address)
		assert.Equal(t, origListener.ClusterAddress, decListener.ClusterAddress)
		assert.Equal(t, origListener.TLSDisable, decListener.TLSDisable)
		assert.Equal(t, origListener.TLSCertFile, decListener.TLSCertFile)
		assert.Equal(t, origListener.TLSKeyFile, decListener.TLSKeyFile)
	}
}

func TestDecodeHCLInvalidInput(t *testing.T) {
	// Test with invalid HCL
	invalidHCL := []byte("this is not valid HCL {{{")
	config, err := DecodeHCL(invalidHCL, "config.hcl")
	assert.Error(t, err, "Should return error for invalid HCL")
	assert.Nil(t, config, "Config should be nil on error")

	// Test with empty input
	emptyHCL := []byte("")
	config, err = DecodeHCL(emptyHCL, "config.hcl")
	// Empty HCL might be valid but result in empty config
	if err == nil {
		assert.NotNil(t, config, "Config should not be nil if no error")
	}
}
