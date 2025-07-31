package maintenance

import (
	"fmt"
	"os"
)

// getMaintenanceURL reads the MAINTENANCE_URL environment variable.
func getMaintenanceURL() (string, error) {
	const envKey = "MAINTENANCE_URL"
	if url := os.Getenv(envKey); url != "" {
		return url, nil
	}
	return "", fmt.Errorf("%s environment variable not set", envKey)
}
