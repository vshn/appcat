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

func TestForgejoWebhookHandler_ValidateCreate_DeniedMailerProtocol(t *testing.T) {
	ctx := context.TODO()
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		Build()

	handler := ForgejoWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNForgejo{},
			name:       "forgejo",
			nameLength: 30,
		},
	}

	newForgejo := func(pinImageTag string) *vshnv1.VSHNForgejo {
		return &vshnv1.VSHNForgejo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myinstance",
				Namespace: "testns",
			},
			Spec: vshnv1.VSHNForgejoSpec{
				Parameters: vshnv1.VSHNForgejoParameters{
					Service: vshnv1.VSHNForgejoServiceSpec{
						FQDN: []string{"myforgejo.example.tld"},
						ForgejoSettings: vshnv1.VSHNForgejoSettings{
							Config: vshnv1.VSHNForgejoConfig{
								Mailer: map[string]string{
									"PROTOCOL": "sendmail",
								},
							},
						},
					},
					Maintenance: vshnv1.VSHNDBaaSMaintenanceScheduleSpec{
						PinImageTag: pinImageTag,
					},
				},
			},
		}
	}

	t.Log("denied mailer protocol without pinImageTag: expect rejection")
	_, err := handler.ValidateCreate(ctx, newForgejo(""))
	assert.Error(t, err)
	assert.ErrorContains(t, err, "bad mailer.PROTOCOL")

	t.Log("denied mailer protocol with pinImageTag set: expect rejection (no bypass)")
	_, err = handler.ValidateCreate(ctx, newForgejo("forgejo:1.0.0"))
	assert.Error(t, err)
	assert.ErrorContains(t, err, "bad mailer.PROTOCOL")
}

func Test_validateForgejoConfig(t *testing.T) {
	tests := []struct {
		name    string
		mailer  map[string]string
		wantErr bool
	}{
		{name: "NoMailer"},
		{name: "SMTP", mailer: map[string]string{"ENABLED": "true", "PROTOCOL": "smtp", "SMTP_ADDR": "mail.example.com"}},
		{name: "SMTPS", mailer: map[string]string{"PROTOCOL": "smtps", "SMTP_ADDR": "mail.example.com", "SMTP_PORT": "465"}},
		{name: "StartTLS", mailer: map[string]string{"protocol": "smtp+starttls"}},
		{name: "Dummy", mailer: map[string]string{"PROTOCOL": "dummy"}},
		{name: "NoProtocol", mailer: map[string]string{"SMTP_ADDR": "mail.example.com"}},
		{name: "Sendmail", mailer: map[string]string{"PROTOCOL": "sendmail"}, wantErr: true},
		{name: "UnixSocket", mailer: map[string]string{"PROTOCOL": "smtp+unix"}, wantErr: true},
		{name: "ProtocolKeyWithSpace", mailer: map[string]string{"PROTOCOL ": "sendmail"}, wantErr: true},
		{name: "ProtocolValueWithSpace", mailer: map[string]string{"PROTOCOL": " sendmail"}, wantErr: true},
		{name: "MailerType", mailer: map[string]string{"MAILER_TYPE": "sendmail"}, wantErr: true},
		{name: "SendmailPath", mailer: map[string]string{"SENDMAIL_PATH": "/usr/bin/true"}, wantErr: true},
		{name: "SMTPAddrSocketPath", mailer: map[string]string{"SMTP_ADDR": "/run/mail.sock"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateForgejoConfig(vshnv1.VSHNForgejoSettings{Config: vshnv1.VSHNForgejoConfig{Mailer: tt.mailer}})
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
