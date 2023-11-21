package common

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
)

func GetResources(size *vshnv1.VSHNSizeSpec, r utils.Resources) (string, string, string, string, string) {
	reqMem := size.Requests.Memory
	reqCPU := size.Requests.CPU
	mem := size.Memory
	cpu := size.CPU
	disk := size.Disk

	if reqMem == "" {
		reqMem = r.MemoryRequests.String()
	}
	if reqCPU == "" {
		reqCPU = r.CPURequests.String()
	}
	if mem == "" {
		mem = r.MemoryLimits.String()
	}
	if cpu == "" {
		cpu = r.CPULimits.String()
	}
	if disk == "" {
		disk = r.Disk.String()
	}
	return reqMem, reqCPU, mem, cpu, disk
}
