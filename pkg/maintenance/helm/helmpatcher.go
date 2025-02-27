package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	"github.com/vshn/appcat/v4/pkg/common/jsonpatch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ImageContentType name of the image type in the docker registry API
	ImageContentType = "image"
	// ActiveTagStatus indicates that the image is active
	ActiveTagStatus = "active"
)

// ImagePatcher is a generic maintenance runner for helm based services
// It can handle semver images as well as images that don't adhere to semver.
type ImagePatcher struct {
	k8sClient         client.Client
	httpClient        *http.Client
	log               logr.Logger
	instanceNamespace string
}

// NewImagePatcher returns a new ImagePatcher
// It will patch the image tag of any given helm release.
func NewImagePatcher(c client.Client, hc *http.Client, l logr.Logger) ImagePatcher {
	return ImagePatcher{
		k8sClient:  c,
		httpClient: hc,
		log:        l,
	}
}

// ValuePath describes a generic path within a release's values.
// It can generate various different representations of the fields, depending on
// the use case.
type ValuePath struct {
	path []string
}

// VersionLister provides a list of viable versions to compare against.
type VersionLister interface {
	GetVersions() []string
}

// GetJSONPatchPath returns the representation of the path as used in for patching the release
func (h *ValuePath) GetJSONPatchPath() string {
	return "/spec/forProvider/values/" + strings.Join(h.path, "/")
}

// AsSlice will return the path as a slice for the useage in with the unstructured module
func (h *ValuePath) AsSlice() []string {
	return h.path
}

// NewValuePath returns a new ValuePath
func NewValuePath(path ...string) ValuePath {
	return ValuePath{
		path: path,
	}
}

// Payload contains the response body of a docker image tags
type Payload struct {
	Results []Result `json:"results,omitempty"`
}

// GetVersions returns all found image tags from the payload
func (p *Payload) GetVersions() []string {
	versions := []string{}
	for _, v := range p.Results {
		if v.TagStatus == ActiveTagStatus && v.ContentType == ImageContentType {
			versions = append(versions, v.Name)
		}
	}
	return versions
}

// RegistryResult contains the result for querying a v2 docker registry
type RegistryResult struct {
	Name string   `json:"name,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

// GetVersions returns the underlying list of versions
func (v RegistryResult) GetVersions() []string {
	return v.Tags
}

// Result is the object that has details of a docker image tag
type Result struct {
	// Name is the tag
	Name string `json:"name,omitempty"`
	// TagStatus indicates if the tag is active
	TagStatus string `json:"tag_status,omitempty"`
	// ContentType indicates whether this Result is an image
	ContentType string `json:"content_type,omitempty"`
}

// VersionComparisonStrategy is a function that actually determines if a given release needs to be updated
// The returned string should contain the latest version, or the same version, if there's no latest.
type VersionComparisonStrategy func(VersionLister, string) (string, error)

// DoMaintenance will run a helm semver maintenance script.
// tagURL should be the url of the docker repository where the image resides. It has to be publicly available.
// The expected format of the url is `https://hub.docker.com/v2/repositories/myorg/myservice/tags/?page_size=100`
// tagPath should be the specific path in the helm values where the tag is specified. It must not be nil and needs to be initialized at least with an empty string.
// versionComp contains the function to compare if a given list of versions actually contains a valid new version or not.
func (h *ImagePatcher) DoMaintenance(ctx context.Context, tagURL string, tagPath ValuePath, versionComp VersionComparisonStrategy) error {
	err := h.configure()
	if err != nil {
		return fmt.Errorf("cannot configure job to run: %v", err)
	}

	h.log = h.log.WithValues("instanceNamespace", h.instanceNamespace)

	h.log.Info("Starting maintenance on instance")

	h.log.Info("Getting current versions")
	results, err := h.getVersions(tagURL)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %v", tagURL, err)
	}

	if len(results.GetVersions()) == 0 {
		return fmt.Errorf("no images found! Please check url")
	}

	h.log.Info("Getting release")
	release, err := GetRelease(ctx, h.k8sClient, h.instanceNamespace)
	if err != nil {
		return fmt.Errorf("cannot get release from namespace %s: %v", h.instanceNamespace, err)
	}

	h.log.Info("Found release " + release.Name)

	h.log.Info("Getting current image tag, from path", "fieldPath", strings.Join(tagPath.AsSlice(), ","))

	currentTag, err := h.getCurrentTagFromRelease(release, tagPath.AsSlice())
	if err != nil {
		return fmt.Errorf("cannot get current tag from release: %w", err)
	}

	h.log.Info("Getting latest image version")
	version, err := versionComp(results, currentTag)
	if err != nil {
		return fmt.Errorf("cannot parse new version from release %s: %v", release.Name, err)
	}

	if currentTag == version {
		h.log.Info("Release has already the latest version, skipping maintenance")
		return nil
	}

	h.log.Info("Patching release with version " + version)
	err = h.patchRelease(ctx, version, release, tagPath.GetJSONPatchPath())
	if err != nil {
		return fmt.Errorf("cannot patch release %s: %v", release.Name, err)
	}
	return nil
}

func (h *ImagePatcher) patchRelease(ctx context.Context, version string, release *v1beta1.Release, patchPath string) error {
	patchOps := []jsonpatch.JSONpatch{
		{
			Op:    jsonpatch.JSONopReplace,
			Path:  patchPath,
			Value: version,
		},
	}
	patch, err := json.Marshal(patchOps)
	if err != nil {
		return fmt.Errorf("can't marshal patch: %v", err)
	}
	return h.k8sClient.Patch(ctx, release, client.RawPatch(types.JSONPatchType, patch))
}

func (h *ImagePatcher) getCurrentTagFromRelease(release *v1beta1.Release, fieldPath []string) (string, error) {
	values := &map[string]interface{}{}
	err := json.Unmarshal(release.Spec.ForProvider.Values.Raw, values)
	if err != nil {
		return "", fmt.Errorf("cannot unmarshal values from release %s: %v", release.Name, err)
	}

	tag, b, err := unstructured.NestedFieldCopy(*values, fieldPath...)
	if err != nil || !b {
		return "", fmt.Errorf("cannot get image tag from release %s: %v", release.Name, err)
	}
	return tag.(string), nil
}

// GetRelease will get the first release it finds in the given instanceNamespace
func GetRelease(ctx context.Context, k8sClient client.Client, instanceNamespace string) (*v1beta1.Release, error) {
	rl := &v1beta1.ReleaseList{}
	err := k8sClient.List(ctx, rl)
	if err != nil {
		return nil, fmt.Errorf("cannot get release list from cluster: %v", err)
	}

	release := v1beta1.Release{}
	for _, item := range rl.Items {
		if item.Spec.ForProvider.Namespace == instanceNamespace {
			release = item
			break
		}
	}
	if release.Name == "" {
		return nil, fmt.Errorf("cannot find release in namespace %s", instanceNamespace)
	}
	return &release, nil
}

func (h *ImagePatcher) getVersions(imageURL string) (VersionLister, error) {
	// We're using docker hub's rich api to list the image tags, if it's on docker hub.
	if strings.Contains(imageURL, "https://hub.docker.com") {
		return h.getHubVersions(imageURL)
	}
	// For the rest we use the default registry API.
	return h.getRegistryVersions(imageURL)
}

func (h *ImagePatcher) getRegistryVersions(imageURL string) (VersionLister, error) {
	results := &RegistryResult{}

	resp, err := h.httpClient.Get(imageURL)
	if err != nil {
		return nil, fmt.Errorf("cannot access registry: %w", err)
	}

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned bad status code (%d): %s", resp.StatusCode, string(b))
	}

	err = json.NewDecoder(resp.Body).Decode(results)
	if err != nil {
		return nil, fmt.Errorf("cannot decode docker registry resuls: %w", err)
	}

	return results, nil
}

func (h *ImagePatcher) getHubVersions(imageURL string) (VersionLister, error) {
	results := make([]Result, 0)
	page := 1
	for {
		resp, err := h.httpClient.Get(imageURL + "&page=" + strconv.Itoa(page))
		if err != nil {
			return nil, fmt.Errorf("cannot access docker hub %s: %v", imageURL, err)
		}

		payload := &Payload{}
		err = json.NewDecoder(resp.Body).Decode(payload)
		if err != nil {
			return nil, fmt.Errorf("cannot decode payload from %s: %v", imageURL, err)
		}
		results = append(results, payload.Results...)
		if len(payload.Results) != 100 {
			break
		}
		page++
	}

	return &Payload{Results: results}, nil
}

func (h *ImagePatcher) configure() error {
	errString := "missing environment variable: %s"
	h.instanceNamespace = viper.GetString("INSTANCE_NAMESPACE")
	if h.instanceNamespace == "" {
		return fmt.Errorf(errString, "INSTANCE_NAMESPACE")
	}
	return nil
}

// SemVerPatchesOnly is a VersionComparisonStrategy that will determine if the latest version as long as it adheres to semver
// It will only update the patch part of the version, it will not consider the Major or Minor version.
// It's able to either ignore the build information appended to the semver version, or not.
func SemVerPatchesOnly(ignoreBuild bool) func(results VersionLister, currentTag string) (string, error) {
	return func(results VersionLister, currentTag string) (string, error) {
		currentV, err := semver.ParseTolerant(currentTag)
		if err != nil {
			return "", fmt.Errorf("current version %s of release is not sem ver: %v", currentTag, err)
		}

		newV := currentV
		for _, res := range results.GetVersions() {
			v, err := semver.New(res)
			if err != nil {
				continue
			}
			if v.Major == newV.Major && v.Minor == newV.Minor && v.Patch > newV.Patch {
				if ignoreBuild && v.Pre != nil {
					continue
				}
				newV = *v
			}
		}
		if currentV.EQ(newV) {
			// Don't return the semver representation! Sometimes there might be
			// a v1.0 but no v1.0.0 tag for an image (AKA Bitnami doing an oopsie).
			return currentTag, nil
		}
		return newV.String(), nil
	}
}
