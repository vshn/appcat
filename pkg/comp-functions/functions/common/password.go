package common

import (
	"fmt"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha2"
	"github.com/sethvargo/go-password/password"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddCredentialsSecret creates secrets and passwords for use with helm based services.
// This is to avoid issues with re-generating passwords if helm internal password generators are used.
// The function accepts a list of fields that should be populated with passwords.
// It returns the name of the secret resource, so it can be referenced later. The name of the inner secret object is the
// same as the resource name.
// Additionally it exposes the generated passwords as connection details, for easier retrieval.
func AddCredentialsSecret(comp InfoGetter, svc *runtime.ServiceRuntime, fieldList []string) (string, error) {
	return AddGenericSecret(comp, svc, "credentials-secret", fieldList)
}

// AddGenericSecret generates passwords the same way AddCredentialsSecret does.
// With the difference that the resource name can be chosen.
// This is helpful if multiple different random generated passwords are necessary.
func AddGenericSecret(comp InfoGetter, svc *runtime.ServiceRuntime, suffix string, fieldList []string) (string, error) {
	secretObjectName := runtime.EscapeDNS1123(comp.GetName()+"-"+suffix, false)
	secret := &corev1.Secret{}
	cd := []xkube.ConnectionDetail{}
	err := svc.GetObservedKubeObject(secret, secretObjectName)
	if err == runtime.ErrNotFound {
		stringData := map[string]string{}

		for _, field := range fieldList {
			stringData[field], err = genPassword()
			if err != nil {
				return secretObjectName, fmt.Errorf("cannot generate pw for %s: %w", field, err)
			}
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretObjectName,
				Namespace: comp.GetInstanceNamespace(),
			},
			StringData: stringData,
		}
	} else if err != nil {
		return secretObjectName, err
	}

	// We need to add the secrets every time, or we override existing ones with
	// an empty array.
	for _, field := range fieldList {
		cd = append(cd, xkube.ConnectionDetail{
			ObjectReference: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  comp.GetInstanceNamespace(),
				Name:       secretObjectName,
				FieldPath:  "data." + field,
			},
			ToConnectionSecretKey: field,
		})
	}

	return secretObjectName, svc.SetDesiredKubeObject(secret, secretObjectName, runtime.KubeOptionAddConnectionDetails(comp.GetInstanceNamespace(), cd...))
}

func genPassword() (string, error) {
	gen, err := password.NewGenerator(&password.GeneratorInput{})
	if err != nil {
		return "", err
	}

	return gen.Generate(16, 4, 0, false, true)
}
