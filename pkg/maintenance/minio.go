package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	minioURL        = "https://hub.docker.com/v2/repositories/minio/minio/tags/?page_size=100"
	dateRegex       = minioTagPrefix + `(?P<date>.*Z$)`
	minioTimeLayout = "2006-01-02T15-04-05Z07:00"
	minioTagPrefix  = "RELEASE."
	earliestTag     = "2023-09-20T22-49-55Z"
)

// Minio contains all necessary dependencies to successfully run a minio maintenance
type Minio struct {
	k8sClient  client.Client
	httpClient *http.Client
	log        logr.Logger
}

// NewMinio returns a new Minio object
func NewMinio(c client.Client, hc *http.Client) Minio {
	return Minio{
		k8sClient:  c,
		httpClient: hc,
	}
}

// DoMaintenance will run minios's maintenance script.
func (m *Minio) DoMaintenance(ctx context.Context) error {
	m.log = logr.FromContextOrDiscard(ctx).WithValues("type", "minio")
	patcher := helm.NewImagePatcher(m.k8sClient, m.httpClient, m.log)

	valuesPath := helm.NewValuePath("image", "tag")

	err := m.ensureTagIsNotNil(ctx, valuesPath)
	if err != nil {
		return err
	}

	return patcher.DoMaintenance(ctx, minioURL, valuesPath, compareMinioVersions)
}

// compareMinioVersions specifically checks for new Minio versions
// Minio has a pretty unique rolling release cycle which is based on the date.
// Each Minio image tag contains the date when it was created.
// The form of the tags is like this: `RELEASE.2023-09-16T01-01-47Z`
func compareMinioVersions(results []helm.Result, currentTag string) (string, error) {

	if currentTag == "" {
		currentTag = minioTagPrefix + earliestTag
	}

	currentDate, err := parseMinioDate(currentTag)
	if err != nil {
		currentDate, err = parseMinioDate(minioTagPrefix + earliestTag)
		if err != nil {
			return "", err
		}
	}

	newDate := currentDate

	for _, res := range results {
		if res.TagStatus == helm.ActiveTagStatus && res.ContentType == helm.ImageContentType {
			v, err := parseMinioDate(res.Name)
			if err != nil {
				continue
			}
			if newDate.Before(v) {
				newDate = v
			}
		}
	}

	newTag := minioTagPrefix + newDate.Format(minioTimeLayout)

	return newTag, nil
}

func parseMinioDate(currentTag string) (time.Time, error) {
	r := regexp.MustCompile(dateRegex)
	matches := r.FindStringSubmatch(currentTag)
	if len(matches) == 0 {
		return time.Now(), fmt.Errorf("did not match any date")
	}
	dateIndex := r.SubexpIndex("date")
	date := matches[dateIndex]
	return time.Parse(minioTimeLayout, date)
}

func (m *Minio) ensureTagIsNotNil(ctx context.Context, valuesPath helm.ValuePath) error {
	instanceNamespace := viper.GetString("INSTANCE_NAMESPACE")
	if instanceNamespace == "" {
		return fmt.Errorf("missing environment variable: %s", "INSTANCE_NAMESPACE")
	}

	m.log.Info("Ensuring that the release contains the tag path in values")

	release, err := helm.GetRelease(ctx, m.k8sClient, instanceNamespace)
	if err != nil {
		return err
	}

	rawValues := release.Spec.ForProvider.Values

	values := map[string]interface{}{}

	err = json.Unmarshal(rawValues.Raw, &values)
	if err != nil {
		return err
	}

	_, found, err := unstructured.NestedString(values, valuesPath.AsSlice()...)
	if err != nil {
		return err
	}

	if !found {

		m.log.Info("No tag found, adding empty tag")

		err := unstructured.SetNestedField(values, "", valuesPath.AsSlice()...)
		if err != nil {
			return err
		}

		rawValues, err := json.Marshal(values)
		if err != nil {
			return err
		}

		release.Spec.ForProvider.Values.Raw = rawValues

		return m.k8sClient.Update(ctx, release)

	}

	m.log.Info("Release already contains a tag")

	return nil
}
