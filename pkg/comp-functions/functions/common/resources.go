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

func GetResources(size *vshnv1.VSHNSizeSpec, plan utils.Resources) (Resources, error) {
	reqMem := resource.Quantity{}
	reqCPU := resource.Quantity{}
	mem := resource.Quantity{}
	cpu := resource.Quantity{}
	disk := plan.Disk

	var err error

	if size.Requests.Memory != "" {
		reqMem, err = resource.ParseQuantity(size.Requests.Memory)
		if err != nil {
			return Resources{}, err
		}
	}

	if size.Requests.CPU != "" {
		reqCPU, err = resource.ParseQuantity(size.Requests.CPU)
		if err != nil {
			return Resources{}, err
		}
	}

	if size.Memory != "" {
		mem, err = resource.ParseQuantity(size.Memory)
		if err != nil {
			return Resources{}, err
		}
	}

	if size.CPU != "" {
		cpu, err = resource.ParseQuantity(size.CPU)
		if err != nil {
			return Resources{}, err
		}
	}

	if size.Disk != "" {
		disk, err = resource.ParseQuantity(size.Disk)
		if err != nil {
			return Resources{}, err
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
	}, nil
}

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

func getRequest(req, limit, planRequests resource.Quantity) resource.Quantity {

	finalMem := req

	if req.IsZero() {
		finalMem = planRequests
	}

	if limit.Cmp(finalMem) == -1 {
		finalMem = limit
	}

	return finalMem
}

func getHigherQuantity(a, b resource.Quantity) resource.Quantity {

	if a.Cmp(b) == -1 {
		return b
	}
	return a
}
