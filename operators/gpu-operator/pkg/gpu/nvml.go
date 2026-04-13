package gpu

import (
	"fmt"
	"sync"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

const (
	// Temperature thresholds for GPU health classification
	temperatureDegraded = 85 // degrees Celsius
	temperatureFailed   = 95 // degrees Celsius
)

// GpuInfo represents information about a GPU
type GpuInfo struct {
	Index       int
	UUID        string
	Health      string
	Temperature int
	PowerUsage  int
	MemoryUsed  int
	MemoryTotal int
	Utilization int
}

var (
	nvmlOnce sync.Once
	nvmlErr  nvml.Return
)

func initNVML() error {
	nvmlOnce.Do(func() {
		nvmlErr = nvml.Init()
	})
	if nvmlErr != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %w", fmt.Errorf("%v", nvml.ErrorString(nvmlErr)))
	}
	return nil
}

// GetDriverInfo returns the NVIDIA driver and CUDA versions
func GetDriverInfo() (string, string, error) {
	if err := initNVML(); err != nil {
		return "", "", err
	}

	driverVersion, ret := nvml.SystemGetDriverVersion()
	if ret != nvml.SUCCESS {
		return "", "", fmt.Errorf("failed to get driver version: %v", nvml.ErrorString(ret))
	}

	cudaVersion, ret := nvml.SystemGetCudaDriverVersion()
	if ret != nvml.SUCCESS {
		return "", "", fmt.Errorf("failed to get CUDA version: %v", nvml.ErrorString(ret))
	}

	cudaVersionStr := fmt.Sprintf("%d.%d", cudaVersion/1000, (cudaVersion%1000)/10)

	return driverVersion, cudaVersionStr, nil
}

// GetGpuInfo returns information about all GPUs
func GetGpuInfo(expectedCount int) ([]GpuInfo, error) {
	if err := initNVML(); err != nil {
		return nil, err
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	if count != expectedCount {
		return nil, fmt.Errorf("expected %d GPUs but found %d", expectedCount, count)
	}

	gpuInfoList := make([]GpuInfo, 0, count)

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get device %d: %v", i, nvml.ErrorString(ret))
		}

		info, err := getDeviceInfo(device, i)
		if err != nil {
			return nil, fmt.Errorf("failed to get info for device %d: %w", i, err)
		}

		gpuInfoList = append(gpuInfoList, info)
	}

	return gpuInfoList, nil
}

func getDeviceInfo(device nvml.Device, index int) (GpuInfo, error) {
	info := GpuInfo{
		Index:  index,
		Health: "Unknown",
	}

	// Get UUID
	uuid, ret := device.GetUUID()
	if ret != nvml.SUCCESS {
		return info, fmt.Errorf("failed to get UUID: %w", fmt.Errorf("%v", nvml.ErrorString(ret)))
	}
	info.UUID = uuid

	// Get temperature
	tempRetrieved := false
	temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret == nvml.SUCCESS {
		info.Temperature = int(temp)
		tempRetrieved = true
	}

	// Get power usage
	power, ret := device.GetPowerUsage()
	if ret == nvml.SUCCESS {
		info.PowerUsage = int(power / 1000) // Convert from milliwatts to watts
	}

	// Get memory info
	memory, ret := device.GetMemoryInfo()
	if ret == nvml.SUCCESS {
		info.MemoryUsed = int(memory.Used / 1024 / 1024)   // Convert to MB
		info.MemoryTotal = int(memory.Total / 1024 / 1024) // Convert to MB
	}

	// Get utilization
	utilization, ret := device.GetUtilizationRates()
	if ret == nvml.SUCCESS {
		info.Utilization = int(utilization.Gpu)
	}

	// Check health based on temperature
	if tempRetrieved {
		if info.Temperature > temperatureFailed {
			info.Health = "Failed"
		} else if info.Temperature > temperatureDegraded {
			info.Health = "Degraded"
		} else {
			info.Health = "Healthy"
		}
	}

	return info, nil
}
