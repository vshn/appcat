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

	// Deploy OpenBao and all related resources
	assert.Nil(t, DeployOpenBao(ctx, comp, svc))

	ns := &corev1.Namespace{}
	assert.NoError(t, svc.GetObservedKubeObject(ns, comp.Name+"-ns"))

	// Test HCL config secret creation
	configSecretName := comp.Name + "-config"
	secret := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(secret, configSecretName))

	// Verify secret has the expected namespace
	assert.Equal(t, comp.GetInstanceNamespace(), secret.Namespace)
	assert.Equal(t, configSecretName, secret.Name)

	// Verify secret contains the HCL config data
	require.Contains(t, secret.Data, "config.hcl", "Secret should contain config.hcl key")
	hclBytes := secret.Data["config.hcl"]
	require.NotEmpty(t, hclBytes, "HCL config should not be empty")

	// Parse and validate the HCL configuration using the DecodeHCL helper
	parsedConfig, err := DecodeHCL(hclBytes, "config.hcl")
	require.NoError(t, err, "HCL should decode properly")
	require.NotNil(t, parsedConfig, "Parsed config should not be nil")

	// Verify the configuration values
	assert.True(t, parsedConfig.UI, "UI should be enabled")
	assert.Equal(t, "info", parsedConfig.LogLevel)
	assert.Equal(t, "json", parsedConfig.LogFormat)
	assert.Equal(t, comp.GetServiceName(), parsedConfig.ClusterName, "ClusterName should match service name")
	assert.Equal(t, "https://openbao:8200", parsedConfig.APIAddr)
	assert.Equal(t, "https://openbao:8201", parsedConfig.ClusterAddr)
	assert.Equal(t, "/var/run/openbao.pid", parsedConfig.PidFile)

	// Verify listener configuration
	require.Len(t, parsedConfig.Listeners, 1, "Should have exactly one listener")
	listener := parsedConfig.Listeners[0]
	assert.Equal(t, "tcp", listener.Type)
	assert.Equal(t, "0.0.0.0:8200", listener.Address)
	assert.Equal(t, "0.0.0.0:8201", listener.ClusterAddress)
	assert.False(t, listener.TLSDisable, "TLS should be enabled")
	assert.NotEmpty(t, listener.TLSCertFile, "TLS cert file should be set")
	assert.NotEmpty(t, listener.TLSKeyFile, "TLS key file should be set")
	assert.Equal(t, "tls12", listener.TLSMinVersion)
}

// TestEncodeDecodeHCL tests the EncodeHCL and DecodeHCL helper functions
func TestEncodeDecodeHCL(t *testing.T) {
	// Create a test config
	originalConfig := NewOpenBaoConfig(
		WithDefaultOptions(),
		WithClusterName("test-cluster"),
	)

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
	assert.Equal(t, originalConfig.PidFile, decodedConfig.PidFile)
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
		assert.Equal(t, origListener.TLSMinVersion, decListener.TLSMinVersion)
	}
}

// TestDecodeHCLInvalidInput tests DecodeHCL with invalid input
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
