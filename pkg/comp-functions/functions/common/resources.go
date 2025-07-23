package common

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Resources struct {
	ReqMem resource.Quantity
	ReqCPU resource.Quantity
	Mem    resource.Quantity
	CPU    resource.Quantity
	Disk   resource.Quantity
}

// GetResources will return a `Resources` object with the correctly calculated requests,
// limits and disk space according to the definitions in the plan as well as the overrides
// in the claim.
func GetResources(size *vshnv1.VSHNSizeSpec, plan utils.Resources) (Resources, []error) {
	reqMem := resource.Quantity{}
	reqCPU := resource.Quantity{}
	mem := resource.Quantity{}
	cpu := resource.Quantity{}
	disk := plan.Disk

	var errors []error
	var err error

	if size.Requests.Memory != "" {
		reqMem, err = resource.ParseQuantity(size.Requests.Memory)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if size.Requests.CPU != "" {
		reqCPU, err = resource.ParseQuantity(size.Requests.CPU)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if size.Memory != "" {
		mem, err = resource.ParseQuantity(size.Memory)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if size.CPU != "" {
		cpu, err = resource.ParseQuantity(size.CPU)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if size.Disk != "" {
		disk, err = resource.ParseQuantity(size.Disk)
		if err != nil {
			errors = append(errors, err)
		}
	}

	memLimit := getLimit(reqMem, mem, plan.MemoryLimits)
	memReq := getRequest(reqMem, memLimit, plan.MemoryRequests)

	cpuLimit := getLimit(reqCPU, cpu, plan.CPULimits)
	cpuReq := getRequest(reqCPU, cpuLimit, plan.CPURequests)

	return Resources{
		ReqMem: memReq,
		ReqCPU: cpuReq,
		Mem:    memLimit,
		CPU:    cpuLimit,
		Disk:   disk,
	}, errors
}

// getLimit will compare a given limit and request as well as the limit defined in the plan
// and return the appropriate limit.
// The limit is chosen based on the following logic:
//   - If claim contains a limit but no request: Use the limit from the claim  as the final limit
//   - If claim contains a limit and a request: Use the higher of both as the final limit
//     This avoids having higher requests then limits, which is not supported in k8s.
//   - If claim contains only request: Use the higher of request and limit in the plan as the final limit
//     this avoids having higher requests then limits, which is not supported in k8s.
//   - If no limit or request is defined in the claim, use the plans limit as the final limit.
func getLimit(req, limit, planLimit resource.Quantity) resource.Quantity {

	finalMem := resource.Quantity{}

	if !limit.IsZero() && req.IsZero() {
		finalMem = limit
	} else if !limit.IsZero() && !req.IsZero() {
		finalMem = getHigherQuantity(req, limit)
	} else if limit.IsZero() && !req.IsZero() {
		finalMem = getHigherQuantity(req, planLimit)
	} else {
		finalMem = planLimit
	}
	return finalMem
}

// getRequest will compare a given limit and request as well as the limit defined in the plan
// and return the appropriate request.
// This function assumes, that the given limit is already calculated with the `getLimit` function
// and passed to this function.
// The request is chosen based on the following logic:
//   - If no requests is defined in the claim, use the plans request as the final requests.
//   - If the claim contains a limt and request, use the lower of both as the final request.
//     This avoids having higher requests then limits, which is not supported in k8s.
func getRequest(req, limit, planRequests resource.Quantity) resource.Quantity {

	finalReq := req

	if req.IsZero() {
		finalReq = planRequests
	}

	if limit.Cmp(finalReq) == -1 {
		finalReq = limit
	}

	return finalReq
}

// getHigherQuantity will compare the given quantities and return the higher one.
func getHigherQuantity(a, b resource.Quantity) resource.Quantity {

	if a.Cmp(b) == -1 {
		return b
	}
	return a
}

// GetBitnamiNano returns a "nano" bitnami resource termplate, but without the
// ephemeral storage.
// See for more details: https://github.com/bitnami/charts/blob/main/bitnami/common/templates/_resources.tpl#L15
func GetBitnamiNano() map[string]any {
	return map[string]any{
		"requests": map[string]string{
			"cpu":    "100m",
			"memory": "128Mi",
		},
		"limits": map[string]string{
			"cpu":    "150m",
			"memory": "192Mi",
		},
	}
}

func MergeSidecarsIntoValues(values map[string]any, sidecars *utils.Sidecars) {
	for name, sidecar := range *sidecars {
		// Ensure the top-level key exists
		entry, ok := values[name].(map[string]any)
		if !ok {
			entry = map[string]any{}
			values[name] = entry
		}

		// Ensure a "resources" map exists
		resources, ok := entry["resources"].(map[string]any)
		if !ok {
			resources = map[string]any{}
			entry["resources"] = resources
		}

		// Merge Limits
		limits, ok := resources["limits"].(map[string]any)
		if !ok {
			limits = map[string]any{}
			resources["limits"] = limits
		}
		if sidecar.Limits.CPU != "" {
			limits["cpu"] = sidecar.Limits.CPU
		}
		if sidecar.Limits.Memory != "" {
			limits["memory"] = sidecar.Limits.Memory
		}

		// Merge Requests
		requests, ok := resources["requests"].(map[string]any)
		if !ok {
			requests = map[string]any{}
			resources["requests"] = requests
		}
		if sidecar.Requests.CPU != "" {
			requests["cpu"] = sidecar.Requests.CPU
		}
		if sidecar.Requests.Memory != "" {
			requests["memory"] = sidecar.Requests.Memory
		}

		// Put back merged maps (in case they were created)
		resources["limits"] = limits
		resources["requests"] = requests
		entry["resources"] = resources
		values[name] = entry
	}
}
