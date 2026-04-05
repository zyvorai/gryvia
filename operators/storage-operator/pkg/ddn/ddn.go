package ddn

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kubefabricv1 "github.com/ssahani/kube-fabric/operators/storage-operator/api/v1"
)

// InstallCSIDriver installs the DDN CSI driver
func InstallCSIDriver(ctx context.Context, k8sClient client.Client, storage *kubefabricv1.FabricStorage) error {
	// DDN EXAScaler CSI driver installation logic
	return fmt.Errorf("DDN CSI driver installation not yet fully implemented - use DDN EXAScaler CSI helm chart")
}

// HealthCheck checks DDN storage health
func HealthCheck(endpoint string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf("https://%s/api/health", endpoint)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("DDN health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DDN health check returned status %d", resp.StatusCode)
	}

	return nil
}
