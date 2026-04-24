package tcproute

type TCPRouteConfig struct {
	// Name prefix for all created resources (e.g. "mycomp-ssh")
	ResourceName string
	// Listener name inside XListenerSet (e.g. "ssh", "mysql")
	ListenerName string
	// Target service name + port for the TCPRoute backend
	BackendServiceName string
	BackendServicePort int32
	// Pod-level port for NetworkPolicy (the port pods actually listen on)
	PodListenPort int32
	// Pod selector labels for NetworkPolicy
	PodSelectorLabels map[string]string
	// Instance namespace where TCPRoute + NetworkPolicy are created
	InstanceNamespace string
	// Config key names (default: "sshGatewayNamespace", "sshGateways")
	// Allow override so services can use different config keys if needed
	GatewayNamespaceConfigKey string // default "sshGatewayNamespace"
	GatewaysConfigKey         string // default "sshGateways"
}

func (c *TCPRouteConfig) applyDefaults() {
	if c.GatewayNamespaceConfigKey == "" {
		c.GatewayNamespaceConfigKey = defaultGatewayNamespaceConfigKey
	}
	if c.GatewaysConfigKey == "" {
		c.GatewaysConfigKey = defaultGatewaysConfigKey
	}
}

type ObservedState struct {
	Port             int32
	GatewayName      string
	GatewayNamespace string
	Domain           string // looked up from config
}
