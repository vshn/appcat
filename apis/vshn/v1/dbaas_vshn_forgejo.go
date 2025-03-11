package v1

import (
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workaround to make nested defaulting work.
// kubebuilder is unable to set a {} default
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.size.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.backup.default={})"
//go:generate yq -i e ../../generated/vshn.appcat.vshn.io_vshnforgejoes.yaml --expression "with(.spec.versions[]; .schema.openAPIV3Schema.properties.spec.properties.parameters.properties.security.default={})"

// +kubebuilder:object:root=true

// VSHNForgejo is the API for creating Forgejo instances.
type VSHNForgejo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of a VSHNForgejo.
	Spec VSHNForgejoSpec `json:"spec"`

	// Status reflects the observed state of a VSHNForgejo.
	Status VSHNForgejoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type VSHNForgejoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VSHNForgejo `json:"items,omitempty"`
}

// VSHNForgejoSpec defines the desired state of a VSHNForgejo.
type VSHNForgejoSpec struct {
	// Parameters are the configurable fields of a VSHNForgejo.
	Parameters VSHNForgejoParameters `json:"parameters,omitempty"`

	// WriteConnectionSecretToRef references a secret to which the connection details will be written.
	WriteConnectionSecretToRef LocalObjectReference `json:"writeConnectionSecretToRef,omitempty"`
}

// VSHNForgejoParameters are the configurable fields of a VSHNForgejo.
type VSHNForgejoParameters struct {
	// Service contains Forgejo DBaaS specific properties
	Service VSHNForgejoServiceSpec `json:"service,omitempty"`

	// Size contains settings to control the sizing of a service.
	Size VSHNSizeSpec `json:"size,omitempty"`

	// Scheduling contains settings to control the scheduling of an instance.
	Scheduling VSHNDBaaSSchedulingSpec `json:"scheduling,omitempty"`

	// Backup contains settings to control how the instance should get backed up.
	Backup K8upBackupSpec `json:"backup,omitempty"`

	// Maintenance contains settings to control the maintenance of an instance.
	Maintenance VSHNDBaaSMaintenanceScheduleSpec `json:"maintenance,omitempty"`

	// Security contains settings to control the security of a service.
	Security Security `json:"security,omitempty"`

	// Monitoring contains settings to control the monitoring of a service.
	Monitoring VSHNMonitoring `json:"monitoring,omitempty"`

	// Instances defines the number of instances to run.
	Instances int `json:"instances,omitempty"`
}

// VSHNForgejoServiceSpec contains Forgejo DBaaS specific properties
type VSHNForgejoServiceSpec struct {
	// +kubebuilder:default="gitea@local.domain"
	// +kubebuilder:validation:Pattern="^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
	// AdminEmail contains the email address of the admin user.
	AdminEmail string `json:"adminEmail,omitempty"`

	ForgejoSettings VSHNForgejoSettings `json:"forgejoSettings,omitempty"`

	// FQDN contains the FQDNs array, which will be used for the ingress.
	// If it's not set, no ingress will be deployed.
	// This also enables strict hostname checking for this FQDN.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	FQDN []string `json:"fqdn"`

	// +kubebuilder:validation:Enum="besteffort";"guaranteed"
	// +kubebuilder:default="besteffort"

	// ServiceLevel defines the service level of this service. Either Best Effort or Guaranteed Availability is allowed.
	ServiceLevel VSHNDBaaSServiceLevel `json:"serviceLevel,omitempty"`

	// Version contains supported version of Forgejo.
	// Multiple versions are supported. Defaults to 10.0.0 if not set.
	// +kubebuilder:default="10.0.0"
	MajorVersion string `json:"majorVersion,omitempty"`
}

// VSHNForgejoSettings contains user customizable configurations for Forgejo
// https://forgejo.org/docs/latest/admin/config-cheat-sheet
type VSHNForgejoSettings struct {
	// +kubebuilder:validation:Optional
	// AppName is the application name, used in the page title
	AppName string `json:"APP_NAME,omitempty"`

	// +kubebuilder:validation:Optional
	// Config contains settings to customize the Forgejo instance with
	Config VSHNForgejoConfig `json:"config,omitempty"`
}

type VSHNForgejoConfig struct {
	// +kubebuilder:validation:Optional
	Actions VSHNForgejoActionsConfig `json:"actions,omitempty"`

	// +kubebuilder:validation:Optional
	OpenID VSHNForgejoOpenIdConfig `json:"openid,omitempty"`

	// +kubebuilder:validation:Optional
	Service VSHNForgejoServiceConfig `json:"service,omitempty"`

	// +kubebuilder:validation:Optional
	Mailer VSHNForgejoMailerConfig `json:"mailer,omitempty"`
}

// https://forgejo.org/docs/latest/admin/config-cheat-sheet/#actions-actions
type VSHNForgejoActionsConfig struct {
	// +kubebuilder:validation:Optional
	Enabled bool `json:"ENABLED,omitempty"`
}

// https://forgejo.org/docs/latest/admin/config-cheat-sheet/#openid-openid
type VSHNForgejoOpenIdConfig struct {
	// +kubebuilder:default="true"
	// +kubebuilder:validation:Optional
	EnableSignin bool `json:"ENABLE_OPENID_SIGNIN,omitempty"`

	// +kubebuilder:default="! DISABLE_REGISTRATION"
	// +kubebuilder:validation:Optional
	EnableSignup bool `json:"ENABLE_OPENID_SIGNUP,omitempty"`

	// +kubebuilder:validation:Optional
	WhitelistedUris []string `json:"WHITELISTED_URIS,omitempty"`

	// +kubebuilder:validation:Optional
	BlacklistedUris []string `json:"BLACKLISTED_URIS,omitempty"`
}

// https://forgejo.org/docs/latest/admin/config-cheat-sheet/#service-service
type VSHNForgejoServiceConfig struct {
	// +kubebuilder:validation:Optional
	// Disable registration, after which only admin can create accounts for users.
	DisableRegistration bool `json:"DISABLE_REGISTRATION,omitempty"`

	// +kubebuilder:validation:Optional
	// Enable this to ask for mail confirmation of registration. Requires Mailer to be enabled.
	RegisterEmailConfirm bool `json:"REGISTER_EMAIL_CONFIRM,omitempty"`

	// +kubebuilder:validation:XValidation:rule="REGISTER_EMAIL_CONFIRM == false", message="REGISTER_EMAIL_CONFIRM must be false"
	// +kubebuilder:validation:Optional
	// Enable this to manually confirm new registrations. Requires REGISTER_EMAIL_CONFIRM to be disabled.
	RegisterManualConfirm bool `json:"REGISTER_MANUAL_CONFIRM,omitempty"`

	// +kubebuilder:validation:Optional
	// Enable this to send e-mail to watchers of a repository when something happens, like creating issues. Requires Mailer to be enabled.
	EnableNotifyMail bool `json:"ENABLE_NOTIFY_MAIL,omitempty"`

	// +kubebuilder:validation:Optional
	// Enable this to use captcha validation for registration.
	EnableCaptcha bool `json:"ENABLE_CAPTCHA,omitempty"`

	// +kubebuilder:validation:XValidation:rule="ENABLE_CAPTCHA == true", message="ENABLE_CAPTCHA must be true"
	// +kubebuilder:validation:Optional
	// Enable this to require captcha validation for login. You also must enable ENABLE_CAPTCHA.
	RequireCaptchaForLogin bool `json:"REQUIRE_CAPTCHA_FOR_LOGIN,omitempty"`

	// +kubebuilder:validation:XValidation:rule="ENABLE_CAPTCHA == true", message="ENABLE_CAPTCHA must be true"
	// +kubebuilder:validation:Optional
	// Enable this to force captcha validation even for External Accounts (i.e. GitHub, OpenID Connect, etc). You also must enable ENABLE_CAPTCHA.
	RequireExtRegCaptcha bool `json:"REQUIRE_EXTERNAL_REGISTRATION_CAPTCHA,omitempty"`

	// +kubebuilder:default="image"
	// +kubebuilder:validation:Enum=image;recaptcha;hcaptcha;mcaptcha;cfturnstile
	// +kubebuilder:validation:Optional
	CaptchaType string `json:"CAPTCHA_TYPE,omitempty"`

	// +kubebuilder:validation:Optional
	HCaptchaSecret string `json:"HCAPTCHA_SECRET,omitempty"`

	// +kubebuilder:validation:Optional
	HCaptchaSitekey string `json:"HCAPTCHA_SITEKEY,omitempty"`

	// +kubebuilder:validation:Optional
	MCaptchaSecret string `json:"MCAPTCHA_SECRET,omitempty"`

	// +kubebuilder:validation:Optional
	MCaptchaSitekey string `json:"MCAPTCHA_SITEKEY,omitempty"`

	// +kubebuilder:validation:Optional
	MCaptchaUrl string `json:"MCAPTCHA_URL,omitempty"`
}

// https://forgejo.org/docs/latest/admin/config-cheat-sheet/#mailer-mailer
type VSHNForgejoMailerConfig struct {
	// +kubebuilder:validation:Optional
	Enabled bool `json:"ENABLED,omitempty"`

	// +kubebuilder:validation:Enum=smtp;smtps;smtp+starttls;dummy
	// +kubebuilder:validation:Optional
	// PROTOCOL is the mail server protocol.
	Protocol string `json:"PROTOCOL,omitempty"`

	// +kubebuilder:validation:Optional
	// SMTP_ADDR is the mail server address.
	SmtpAddr string `json:"SMTP_ADDR,omitempty"`

	// +kubebuilder:validation:Optional
	// SMTP_PORT is the mail server port. If no protocol is specified, it will be inferred by this setting.
	SmtpPort int `json:"SMTP_PORT,omitempty"`

	// +kubebuilder:validation:Optional
	// USER is the username of mailing user
	User string `json:"USER,omitempty"`

	// +kubebuilder:validation:Optional
	// FROM is the mail from address, RFC 5322. This can be just an email address, or the “Name” <email@example.com> format.
	From string `json:"FROM,omitempty"`

	// Password is inferred from env FORGEJO__MAILER__PASSWD to allow passing it as a secret
}

// VSHNForgejoSizeSpec contains settings to control the sizing of a service.
type VSHNForgejoSizeSpec struct {

	// CPURequests defines the requests amount of Kubernetes CPUs for an instance.
	CPURequests string `json:"cpuRequests,omitempty"`

	// CPULimits defines the limits amount of Kubernetes CPUs for an instance.
	CPULimits string `json:"cpuLimits,omitempty"`

	// MemoryRequests defines the requests amount of memory in units of bytes for an instance.
	MemoryRequests string `json:"memoryRequests,omitempty"`

	// MemoryLimits defines the limits amount of memory in units of bytes for an instance.
	MemoryLimits string `json:"memoryLimits,omitempty"`

	// Disk defines the amount of disk space for an instance.
	Disk string `json:"disk,omitempty"`

	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`
}

// VSHNForgejoStatus reflects the observed state of a VSHNForgejo.
type VSHNForgejoStatus struct {
	// InstanceNamespace contains the name of the namespace where the instance resides
	InstanceNamespace string `json:"instanceNamespace,omitempty"`
	// Schedules keeps track of random generated schedules, is overwriten by
	// schedules set in the service's spec.
	Schedules VSHNScheduleStatus `json:"schedules,omitempty"`

	// ResourceStatus represents the observed state of a managed resource.
	xpv1.ResourceStatus `json:",inline"`
}

func (v *VSHNForgejo) GetClaimNamespace() string {
	return v.GetLabels()["crossplane.io/claim-namespace"]
}

func (v *VSHNForgejo) GetInstanceNamespace() string {
	return fmt.Sprintf("vshn-forgejo-%s", v.GetName())
}

func (v *VSHNForgejo) SetInstanceNamespaceStatus() {
	v.Status.InstanceNamespace = v.GetInstanceNamespace()
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNForgejo represents the internal composite of this claim
type XVSHNForgejo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   XVSHNForgejoSpec   `json:"spec"`
	Status XVSHNForgejoStatus `json:"status,omitempty"`
}

// XVSHNForgejoSpec defines the desired state of a VSHNForgejo.
type XVSHNForgejoSpec struct {
	// Parameters are the configurable fields of a VSHNForgejo.
	Parameters VSHNForgejoParameters `json:"parameters,omitempty"`

	xpv1.ResourceSpec `json:",inline"`
}

type XVSHNForgejoStatus struct {
	VSHNForgejoStatus   `json:",inline"`
	xpv1.ResourceStatus `json:",inline"`
}

// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true

// XVSHNForgejoList represents a list of composites
type XVSHNForgejoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []XVSHNForgejo `json:"items"`
}

// GetMaintenanceDayOfWeek returns the currently set day of week
func (v *VSHNForgejo) GetMaintenanceDayOfWeek() string {
	if v.Spec.Parameters.Maintenance.DayOfWeek != "" {
		return v.Spec.Parameters.Maintenance.DayOfWeek
	}
	return v.Status.Schedules.Maintenance.DayOfWeek
}

// GetMaintenanceTimeOfDay returns the currently set time of day
func (v *VSHNForgejo) GetMaintenanceTimeOfDay() *TimeOfDay {
	if v.Spec.Parameters.Maintenance.TimeOfDay != "" {
		return &v.Spec.Parameters.Maintenance.TimeOfDay
	}
	return &v.Status.Schedules.Maintenance.TimeOfDay
}

// SetMaintenanceDayOfWeek sets the day of week to the given value
func (v *VSHNForgejo) SetMaintenanceDayOfWeek(dow string) {
	v.Status.Schedules.Maintenance.DayOfWeek = dow
}

// SetMaintenanceTimeOfDay sets the time of day to the given value
func (v *VSHNForgejo) SetMaintenanceTimeOfDay(tod TimeOfDay) {
	v.Status.Schedules.Maintenance.TimeOfDay = tod
}

// GetFullMaintenanceSchedule returns
func (v *VSHNForgejo) GetFullMaintenanceSchedule() VSHNDBaaSMaintenanceScheduleSpec {
	schedule := v.Spec.Parameters.Maintenance
	schedule.DayOfWeek = v.GetMaintenanceDayOfWeek()
	schedule.TimeOfDay = *v.GetMaintenanceTimeOfDay()
	return schedule
}

// GetBackupRetention returns the retention definition for this backup.
func (v *VSHNForgejo) GetBackupRetention() K8upRetentionPolicy {
	return v.Spec.Parameters.Backup.Retention
}

// GetBackupSchedule returns the current backup schedule
func (v *VSHNForgejo) GetBackupSchedule() string {
	if v.Spec.Parameters.Backup.Schedule != "" {
		return v.Spec.Parameters.Backup.Schedule
	}
	return v.Status.Schedules.Backup
}

// SetBackupSchedule overwrites the current backup schedule
func (v *VSHNForgejo) SetBackupSchedule(schedule string) {
	v.Status.Schedules.Backup = schedule
} // GetServiceName returns the name of this service
func (v *VSHNForgejo) GetServiceName() string {
	return "forgejo"
}

// GetPDBLabels returns the labels to be used for the PodDisruptionBudget
// it should match one unique label od pod running in instanceNamespace
// without this, the PDB will match all pods
func (v *VSHNForgejo) GetPDBLabels() map[string]string {
	return map[string]string{}
}

// GetAllowAllNamespaces returns the AllowAllNamespaces field of this service
func (v *VSHNForgejo) GetAllowAllNamespaces() bool {
	return v.Spec.Parameters.Security.AllowAllNamespaces
}

// GetAllowedNamespaces returns the AllowedNamespaces array of this service
func (v *VSHNForgejo) GetAllowedNamespaces() []string {
	if v.Spec.Parameters.Security.AllowedNamespaces == nil {
		v.Spec.Parameters.Security.AllowedNamespaces = []string{}
	}
	return append(v.Spec.Parameters.Security.AllowedNamespaces, v.GetClaimNamespace())
}

func (v *VSHNForgejo) GetSecurity() *Security {
	return &v.Spec.Parameters.Security
}

func (v *VSHNForgejo) GetSize() VSHNSizeSpec {
	return v.Spec.Parameters.Size
}

func (v *VSHNForgejo) GetMonitoring() VSHNMonitoring {
	return v.Spec.Parameters.Monitoring
}

func (v *VSHNForgejo) GetInstances() int {
	return v.Spec.Parameters.Instances
}

func (v *VSHNForgejo) GetBillingName() string {
	return "appcat-" + v.GetServiceName()
}

func (v *VSHNForgejo) GetClaimName() string {
	return v.GetLabels()["crossplane.io/claim-name"]
}

func (v *VSHNForgejo) GetSLA() string {
	return string(v.Spec.Parameters.Service.ServiceLevel)
}

func (v *VSHNForgejo) GetWorkloadName() string {
	if strings.Contains(v.GetName(), "forgejo") {
		return v.GetName()
	}
	return v.GetName() + "-forgejo"
}

func (v *VSHNForgejo) GetWorkloadPodTemplateLabelsManager() PodTemplateLabelsManager {
	return &StatefulSetManager{}
}
