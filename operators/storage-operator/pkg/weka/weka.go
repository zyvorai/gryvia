package weka

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kubefabricv1 "github.com/ssahani/kube-fabric/operators/storage-operator/api/v1"
)

// InstallCSIDriver installs the Weka CSI driver
func InstallCSIDriver(ctx context.Context, k8sClient client.Client, storage *kubefabricv1.FabricStorage) error {
	// Weka CSI driver installation logic
	// Similar to VAST but with Weka-specific configurations
	return fmt.Errorf("Weka CSI driver installation not yet fully implemented - use Helm chart: helm install weka-csi weka/csi-wekafsplugin")
}

// HealthCheck checks Weka cluster health
func HealthCheck(endpoint string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	url := fmt.Sprintf("https://%s/api/v2/healthcheck", endpoint)

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Weka health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Weka health check returned status %d", resp.StatusCode)
	}

	return nil
}
