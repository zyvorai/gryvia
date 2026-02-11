package gpu

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
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

// GetDriverInfo returns the NVIDIA driver and CUDA versions
func GetDriverInfo() (string, string, error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return "", "", fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

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
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

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
		Health: "Healthy",
	}

	// Get UUID
	uuid, ret := device.GetUUID()
	if ret != nvml.SUCCESS {
		return info, fmt.Errorf("failed to get UUID: %v", nvml.ErrorString(ret))
	}
	info.UUID = uuid

	// Get temperature
	temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret == nvml.SUCCESS {
		info.Temperature = int(temp)
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
	if info.Temperature > 85 {
		info.Health = "Degraded"
	}
	if info.Temperature > 95 {
		info.Health = "Failed"
	}

	return info, nil
}

// CheckGpuHealth performs a health check on a specific GPU
func CheckGpuHealth(index int) (string, error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

	device, ret := nvml.DeviceGetHandleByIndex(index)
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("failed to get device: %v", nvml.ErrorString(ret))
	}

	// Check if device is accessible
	_, ret = device.GetUUID()
	if ret != nvml.SUCCESS {
		return "Failed", nil
	}

	// Check temperature
	temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret == nvml.SUCCESS {
		if temp > 95 {
			return "Failed", nil
		}
		if temp > 85 {
			return "Degraded", nil
		}
	}

	return "Healthy", nil
}
