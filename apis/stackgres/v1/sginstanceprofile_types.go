// Package v1 provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v0.0.0-00010101000000-000000000000 DO NOT EDIT.
package v1

import "k8s.io/apimachinery/pkg/runtime"

// SGInstanceProfileSpec defines model for SGInstanceProfileSpec.
type SGInstanceProfileSpec struct {
	// The CPU(s) (cores) and RAM assigned to containers other than patroni container.
	//
	// This section, if left empty, will be filled automatically by the operator with
	//  some defaults that can be proportional to the resources assigned to patroni
	//  container (except for the huge pages that are always left empty).
	Containers runtime.RawExtension `json:"containers,omitempty"`

	// CPU(s) (cores) used for every instance of a SGCluster. The suffix `m`
	//  specifies millicpus (where 1000m is equals to 1).
	//
	// The number of cores set is assigned to the patroni container (that runs both Patroni and PostgreSQL).
	//
	// A minimum of 2 cores is recommended.
	Cpu string `json:"cpu"`

	// RAM allocated for huge pages
	HugePages *SGInstanceProfileSpecHugePages `json:"hugePages,omitempty"`

	// The CPU(s) (cores) and RAM assigned to init containers.
	InitContainers runtime.RawExtension `json:"initContainers,omitempty"`

	// RAM allocated to every instance of a SGCluster. The suffix `Mi` or `Gi`
	//  specifies Mebibytes or Gibibytes, respectively.
	//
	// The amount of RAM set is assigned to the patroni container (that runs both Patroni and PostgreSQL).
	//
	// A minimum of 2-4Gi is recommended.
	Memory string `json:"memory"`

	// On containerized environments, when running production workloads, enforcing container's resources requirements request to be equals to the limits allow to achieve the highest level of performance. Doing so, reduces the chances of leaving
	//  the workload with less resources than it requires. It also allow to set [static CPU management policy](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#static-policy) that allows to guarantee a pod the usage exclusive CPUs on the node.
	//
	// There are cases where you may need to set resource requirement request to a different value than limit. This section allow to do so but requires to enable such feature in the `SGCluster` and `SGDistributedLogs` (see `.spec.nonProductionOptions` section for each of those custom resources).
	Requests *SGInstanceProfileSpecRequests `json:"requests,omitempty"`
}

// SGInstanceProfileSpecHugePages defines model for SGInstanceProfileSpecHugePages.
type SGInstanceProfileSpecHugePages struct {
	// RAM allocated for huge pages with a size of 1Gi. The suffix `Mi` or `Gi`
	//  specifies Mebibytes or Gibibytes, respectively.
	//
	// By default the amount of RAM set is assigned to patroni container
	//  (that runs both Patroni and PostgreSQL).
	Hugepages1Gi *string `json:"hugepages-1Gi,omitempty"`

	// RAM allocated for huge pages with a size of 2Mi. The suffix `Mi` or `Gi`
	//  specifies Mebibytes or Gibibytes, respectively.
	//
	// By default the amount of RAM set is assigned to patroni container
	//  (that runs both Patroni and PostgreSQL).
	Hugepages2Mi *string `json:"hugepages-2Mi,omitempty"`
}

// SGInstanceProfileSpecRequests defines model for SGInstanceProfileSpecRequests.
type SGInstanceProfileSpecRequests struct {
	// The CPU(s) (cores) and RAM assigned to containers other than patroni container.
	Containers runtime.RawExtension `json:"containers,omitempty"`

	// CPU(s) (cores) used for every instance of a SGCluster. The suffix `m`
	//  specifies millicpus (where 1000m is equals to 1).
	//
	// The number of cores set is assigned to the patroni container (that runs both Patroni and PostgreSQL).
	Cpu *string `json:"cpu,omitempty"`

	// The CPU(s) (cores) and RAM assigned to init containers.
	InitContainers runtime.RawExtension `json:"initContainers,omitempty"`

	// RAM allocated to every instance of a SGCluster. The suffix `Mi` or `Gi`
	//  specifies Mebibytes or Gibibytes, respectively.
	//
	// The amount of RAM set is assigned to the patroni container (that runs both Patroni and PostgreSQL).
	Memory *string `json:"memory,omitempty"`
}

type SGInstanceProfileContainer struct {
	Cpu       string                          `json:"cpu,omitempty"`
	Memory    string                          `json:"memory,omitempty"`
	Hugepages *SGInstanceProfileSpecHugePages `json:"hugePages,omitempty"`
}
