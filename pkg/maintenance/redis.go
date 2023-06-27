package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/blang/semver/v4"
	"github.com/crossplane-contrib/provider-helm/apis/release/v1beta1"
	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/pkg/controller/postgres"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

var redisURL = "https://hub.docker.com/v2/namespaces/library/repositories/redis/tags?page_size=100"
var imageContentType = "image"
var activeTagStatus = "active"

// Result is the object that has details of a docker image tag
type Result struct {
	// Name is the tag
	Name string `json:"name,omitempty"`
	// TagStatus indicates if the tag is active
	TagStatus string `json:"tag_status,omitempty"`
	// ContentType indicates whether this Result is an image
	ContentType string `json:"content_type,omitempty"`
}

// Payload contains the response body of a docker image tags
type Payload struct {
	Results []Result `json:"results,omitempty"`
}

type Redis struct {
	K8sClient         client.Client
	HttpClient        *http.Client
	log               logr.Logger
	instanceNamespace string
}

// DoMaintenance will run postgresql's maintenance script.
func (r *Redis) DoMaintenance(ctx context.Context) error {
	err := r.configure()
	if err != nil {
		return fmt.Errorf("cannot configure job to run: %v", err)
	}

	results, err := r.getVersions(redisURL)
	if err != nil {
		return fmt.Errorf("cannot get versions from Docker Hub: %v", err)
	}

	release, err := r.getRelease(ctx)
	if err != nil {
		return fmt.Errorf("cannot get release from namespace %s: %v", r.instanceNamespace, err)
	}

	version, isNewer, err := r.getNewVersion(release, results)
	if err != nil {
		return fmt.Errorf("cannot parse new version from release %s: %v", release.Name, err)
	}
	if !isNewer {
		return nil
	}

	err = r.patchRelease(ctx, *version, release)
	if err != nil {
		return fmt.Errorf("cannot patch release %s: %v", release.Name, err)
	}
	return nil
}

func (r *Redis) patchRelease(ctx context.Context, version semver.Version, release *v1beta1.Release) error {
	patchOps := []postgres.Jsonpatch{
		{
			Op:    "replace",
			Path:  "/spec/forProvider/values/image/tag",
			Value: version.String(),
		},
	}
	patch, err := json.Marshal(patchOps)
	if err != nil {
		return fmt.Errorf("can't marshal patch: %v", err)
	}
	return r.K8sClient.Patch(ctx, release, client.RawPatch(types.JSONPatchType, patch))
}

func (r *Redis) getNewVersion(release *v1beta1.Release, results []Result) (*semver.Version, bool, error) {
	values := &map[string]interface{}{}
	err := json.Unmarshal(release.Spec.ForProvider.Values.Raw, values)
	if err != nil {
		return nil, false, fmt.Errorf("cannot unmarshal values from release %s: %v", release.Name, err)
	}

	tag, b, err := unstructured.NestedFieldCopy(*values, "image", "tag")
	if err != nil || !b {
		return nil, false, fmt.Errorf("cannot get image tag from release %s: %v", release.Name, err)
	}

	currentV, err := semver.ParseTolerant(tag.(string))
	if err != nil {
		return nil, false, fmt.Errorf("current version %s of release is not sem ver: %v", tag, err)
	}

	newV := currentV
	for _, res := range results {
		if res.TagStatus == activeTagStatus && res.ContentType == imageContentType {
			v, err := semver.New(res.Name)
			if err != nil {
				continue
			}
			if v.Major == newV.Major && v.Minor == newV.Minor && v.Patch > newV.Patch && v.Pre == nil {
				newV.Patch = v.Patch
			}
		}
	}
	if currentV.EQ(newV) {
		return &currentV, false, nil
	}
	return &newV, true, nil
}

func (r *Redis) getRelease(ctx context.Context) (*v1beta1.Release, error) {
	rl := &v1beta1.ReleaseList{}
	err := r.K8sClient.List(ctx, rl)
	if err != nil {
		return nil, fmt.Errorf("cannot get release list from cluster: %v", err)
	}

	release := v1beta1.Release{}
	for _, item := range rl.Items {
		if item.Spec.ForProvider.Namespace == r.instanceNamespace {
			release = item
			break
		}
	}
	if release.Name == "" {
		return nil, fmt.Errorf("cannot find Redis release in namespace %s", r.instanceNamespace)
	}
	return &release, nil
}

func (r *Redis) getVersions(redisURL string) ([]Result, error) {
	results := make([]Result, 0)
	page := 1
	for {
		resp, err := r.HttpClient.Get(redisURL + "&page=" + strconv.Itoa(page))
		if err != nil {
			return nil, fmt.Errorf("cannot access docker hub %s: %v", redisURL, err)
		}

		payload := &Payload{}
		err = json.NewDecoder(resp.Body).Decode(payload)
		if err != nil {
			return nil, fmt.Errorf("cannot decode payload from %s: %v", redisURL, err)
		}
		results = append(results, payload.Results...)
		if len(payload.Results) != 100 {
			break
		}
		page++
	}

	return results, nil
}

func (r *Redis) configure() error {
	errString := "missing environment variable: %s"
	r.instanceNamespace = viper.GetString("INSTANCE_NAMESPACE")
	if r.instanceNamespace == "" {
		return fmt.Errorf(errString, "INSTANCE_NAMESPACE")
	}
	return nil
}
