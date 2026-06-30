package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newGarageHandler() GarageWebhookHandler {
	fclient := fake.NewClientBuilder().WithScheme(pkg.SetupScheme()).Build()
	return GarageWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNGarage{},
			name:       "garage",
			gk:         garageGK,
			gr:         garageGR,
			nameLength: maxResourceNameLength,
		},
	}
}

func newGarage(name string, deletionProtection bool) *vshnv1.VSHNGarage {
	return &vshnv1.VSHNGarage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNGarageSpec{
			Parameters: vshnv1.VSHNGarageParameters{
				Security: vshnv1.Security{
					DeletionProtection: deletionProtection,
				},
			},
		},
	}
}

func TestGarageWebhookHandler_ValidateDelete(t *testing.T) {
	ctx := context.TODO()
	handler := newGarageHandler()

	t.Log("Deletion protected instance: expect error")
	_, err := handler.ValidateDelete(ctx, newGarage("my-garage", true))
	assert.Error(t, err)

	t.Log("Deletion protection disabled: expect no error")
	_, err = handler.ValidateDelete(ctx, newGarage("my-garage", false))
	assert.NoError(t, err)
}

func TestGarageWebhookHandler_ValidateCreate(t *testing.T) {
	ctx := context.TODO()
	handler := newGarageHandler()

	t.Log("Valid name: expect no error")
	_, err := handler.ValidateCreate(ctx, newGarage("my-garage", true))
	assert.NoError(t, err)

	t.Log("Name too long: expect error")
	_, err = handler.ValidateCreate(ctx, newGarage("this-garage-instance-name-is-way-too-long-and-should-fail", true))
	assert.Error(t, err)
}

func TestGarageWebhookHandler_ValidateUpdate(t *testing.T) {
	ctx := context.TODO()
	handler := newGarageHandler()

	withDisk := func(disk string) *vshnv1.VSHNGarage {
		g := newGarage("my-garage", false)
		g.Spec.Parameters.Size.Disk = disk
		return g
	}

	t.Log("Disk upsize: expect no error")
	_, err := handler.ValidateUpdate(ctx, withDisk("10Gi"), withDisk("20Gi"))
	assert.NoError(t, err)

	t.Log("Disk downsize: expect error")
	_, err = handler.ValidateUpdate(ctx, withDisk("20Gi"), withDisk("10Gi"))
	assert.Error(t, err)
}
