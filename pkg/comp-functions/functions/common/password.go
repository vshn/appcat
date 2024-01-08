package common

import (
	"fmt"

	"github.com/sethvargo/go-password/password"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InfoGetter will return the name of the given object.
type InfoGetter interface {
	GetName() string
	GetInstanceNamespace() string
}

// AddCredentialsSecret creates secrets and passwords for use with helm based services.
// This is to avoid issues with re-generating passwords if helm internal password generators are used.
// The function accepts a list of fields that should be populated with passwords.
// It returns the name of the secret resource, so it can be referenced later. The name of the inner secret object is the
// same as the resource name.
func AddCredentialsSecret(comp InfoGetter, svc *runtime.ServiceRuntime, fieldList []string) (string, error) {
	secretObjectName := comp.GetName() + "-credentials-secret"
	secret := &corev1.Secret{}
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

	return secretObjectName, svc.SetDesiredKubeObject(secret, secretObjectName)
}

func genPassword() (string, error) {
	gen, err := password.NewGenerator(&password.GeneratorInput{})
	if err != nil {
		return "", err
	}

	return gen.Generate(16, 4, 0, false, true)
}
