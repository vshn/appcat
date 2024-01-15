package common

import (
	"encoding/json"
	"fmt"

	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
)

// GetReleaseValues returns the parsed values from the given release.
func GetReleaseValues(r *xhelmv1.Release) (map[string]interface{}, error) {
	values := map[string]interface{}{}
	if r.Spec.ForProvider.Values.Raw == nil {
		return values, nil
	}
	err := json.Unmarshal(r.Spec.ForProvider.Values.Raw, &values)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal values from release: %v", err)
	}
	return values, nil
}
