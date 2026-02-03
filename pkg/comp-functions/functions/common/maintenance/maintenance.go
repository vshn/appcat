package maintenance

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/blang/semver/v4"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaintenanceResource combines client.Object with initial maintenance status tracking
type MaintenanceResource interface {
	client.Object
	GetInitialMaintenanceRan() bool
	GetInitialMaintenanceCompletedAt() string
	SetInitialMaintenanceStatus(completedAt string, success bool)
}

// Maintenance contains data for maintenance k8s resource generation
type Maintenance struct {
	// instanceNamespace is the namespace where the service pod is running
	instanceNamespace string
	// mainRole is maintenance role name
	mainRole string
	// additionalClusterRoleBinding adds another binding to a cluster role from component
	additionalClusterRoleBinding string
	// service is the service for which this maintenance is supposed to run. Ex:. postgresql
	service string
	// helmBasedService whether the maintenance is for a helm based service
	helmBasedService bool
	// resource object of this service
	resource MaintenanceResource
	// svc is is the ServiceRuntime
	svc *runtime.ServiceRuntime
	// schedule is the schedule spec of the resource
	schedule vshnv1.VSHNDBaaSMaintenanceScheduleSpec
	// policyRules are the permissions to be give to the maintenance role
	policyRules []rbacv1.PolicyRule
	// extraEnvs are extra environment variables to be added to the Cronjob
	extraEnvs []corev1.EnvVar
	// extraResources are extra resources to be added to the default list of resources created by this maintenance
	extraResources []ExtraResource
	// extraKubeOptions are extra options that will get passed to the `SetDesiredKubeObject` function
	extraKubeOptions []runtime.KubeObjectOption
}

// ExtraResource is an extra resource to be added to the desired state of a Crossplane Function IO
type ExtraResource struct {
	Name     string
	Resource client.Object
	Refs     []xkubev1.Reference
}

var (
	maintServiceAccountName = "maintenanceserviceaccount"
	dayOfWeekMap            = map[string]int{
		"monday":    1,
		"tuesday":   2,
		"wednesday": 3,
		"thursday":  4,
		"friday":    5,
		"saturday":  6,
		"sunday":    0,
	}
)

// New creates a Maintenance object with required attributes
func New(r MaintenanceResource, svc *runtime.ServiceRuntime, schedule vshnv1.VSHNDBaaSMaintenanceScheduleSpec, instanceNamespace, service string) *Maintenance {
	return &Maintenance{
		instanceNamespace: instanceNamespace,
		service:           service,
		resource:          r,
		svc:               svc,
		schedule:          schedule,
		extraKubeOptions:  []runtime.KubeObjectOption{},
	}
}

// WithPolicyRules sets the policy rules for the role
func (m *Maintenance) WithPolicyRules(policyRules []rbacv1.PolicyRule) *Maintenance {
	m.policyRules = policyRules
	return m
}

// WithHelmBasedService adds extra environment variables to the cron job
func (m *Maintenance) WithHelmBasedService() *Maintenance {
	m.helmBasedService = true
	m.extraKubeOptions = append(m.extraKubeOptions, runtime.KubeOptionDeployOnControlPlane)
	return m
}

// WithExtraEnvs adds extra environment variables to the cron job
func (m *Maintenance) WithExtraEnvs(extraEnvs ...corev1.EnvVar) *Maintenance {
	m.extraEnvs = extraEnvs
	return m
}

func (m *Maintenance) WithRegistryAuth(username, password string) *Maintenance {
	envVars := []corev1.EnvVar{
		{
			Name:  "REGISTRY_USERNAME",
			Value: username,
		},
		{
			Name:  "REGISTRY_PASSWORD",
			Value: password,
		},
	}
	m.extraEnvs = envVars
	return m
}

// WithExtraResources adds extra resources to the desired composition function
func (m *Maintenance) WithExtraResources(extraResources ...ExtraResource) *Maintenance {
	m.extraResources = extraResources
	return m
}

// WithRole adds namespaced RBAC rules
func (m *Maintenance) WithRole(role string) *Maintenance {
	m.mainRole = role
	return m
}

// WithAdditionalClusterRoleBinding will add an additional role at cluster level
// The role must be created in component appcat and must be passed via composition's config map
func (m *Maintenance) WithAdditionalClusterRoleBinding(clusterrolebinding string) *Maintenance {
	m.additionalClusterRoleBinding = clusterrolebinding
	return m
}

// Run generates k8s resources for maintenance
func (m *Maintenance) Run(ctx context.Context) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)

	log.Info("Adding maintenance cronjob to the instance")
	cron, err := m.parseCron()
	if err != nil {
		return runtime.NewFatalResult(err)
	}
	if cron == "" {
		log.Info("Maintenance schedule not yet populated")
		return runtime.NewNormalResult("Maintenance schedule not yet populated")
	}

	// Handle initial maintenance job
	// Keep the job in desired state until 30 minutes after completion
	if !m.resource.GetInitialMaintenanceRan() {
		// Job hasn't been created yet, create it
		if err := m.createInitialMaintenanceJob(ctx); err != nil {
			log.Error(err, "Failed to create initial maintenance job")
			// Mark as failed
			m.resource.SetInitialMaintenanceStatus(metav1.Now().Format("2006-01-02T15:04:05Z07:00"), false)
			if statusErr := m.svc.SetDesiredCompositeStatus(m.resource); statusErr != nil {
				log.Error(statusErr, "Failed to update InitialMaintenance status after failure")
			}
			return runtime.NewWarningResult(fmt.Sprintf("cannot create initial maintenance job: %v", err))
		}

		// Mark that initial maintenance has been triggered successfully
		m.resource.SetInitialMaintenanceStatus(metav1.Now().Format("2006-01-02T15:04:05Z07:00"), true)
		if err := m.svc.SetDesiredCompositeStatus(m.resource); err != nil {
			log.Error(err, "Failed to update InitialMaintenance status")
			return runtime.NewWarningResult(fmt.Sprintf("cannot update InitialMaintenance status: %v", err))
		}
	} else {
		// Job has been created, check if 30 minutes have passed
		completedAt := m.resource.GetInitialMaintenanceCompletedAt()
		if completedAt != "" {
			completedTime, err := time.Parse("2006-01-02T15:04:05Z07:00", completedAt)
			if err != nil {
				log.Error(err, "Failed to parse initial maintenance completion time, keeping job in desired state")
				// If we can't parse the time, keep the job to be safe
				if err := m.createInitialMaintenanceJob(ctx); err != nil {
					log.Error(err, "Failed to maintain initial maintenance job in desired state")
					return runtime.NewWarningResult(fmt.Sprintf("cannot maintain initial maintenance job: %v", err))
				}
			} else if time.Since(completedTime) > 30*time.Minute {
				// 30 minutes have passed, remove from desired state by not creating it
				log.Info("Initial maintenance job completed more than 30 minutes ago, removing from desired state",
					"completedAt", completedAt,
					"timeSince", time.Since(completedTime).String())
			} else {
				// Still within 30 minutes, keep in desired state
				timeRemaining := 30*time.Minute - time.Since(completedTime)
				log.Info("Keeping initial maintenance job in desired state",
					"completedAt", completedAt,
					"timeRemaining", timeRemaining.String())
				if err := m.createInitialMaintenanceJob(ctx); err != nil {
					log.Error(err, "Failed to maintain initial maintenance job in desired state")
					return runtime.NewWarningResult(fmt.Sprintf("cannot maintain initial maintenance job: %v", err))
				}
			}
		}
	}

	// Helm based services are having maintenance done in a control namespace therefore rbac rules are created
	// once in the component
	if !m.helmBasedService && m.mainRole != "" && m.additionalClusterRoleBinding != "" {
		err = m.createRBAC(ctx)
		if err != nil {
			return runtime.NewFatalResult(err)
		}
	}

	for _, extraR := range m.extraResources {
		err = m.svc.SetDesiredKubeObject(extraR.Resource, extraR.Name, runtime.KubeOptionAddRefs(extraR.Refs...))
		if err != nil {
			return runtime.NewFatalResult(err)
		}
	}

	err = m.createMaintenanceJob(ctx, cron)
	if err != nil {
		return runtime.NewFatalResult(err)
	}
	return nil
}

func (m *Maintenance) createRBAC(ctx context.Context) error {
	err := m.createMaintenanceServiceAccount(ctx)
	if err != nil {
		return fmt.Errorf("can't create maintenance serviceaccount: %v", err)
	}

	err = m.createMaintenanceRole(ctx)
	if err != nil {
		return fmt.Errorf("can't create maintenance role: %v", err)
	}

	err = m.createMaintenanceRoleBinding(ctx)
	if err != nil {
		return fmt.Errorf("can't create maintenance rolebinding: %v", err)
	}

	err = m.createMaintenanceClusterRoleBinding(ctx)
	if err != nil {
		return fmt.Errorf("can't create maintenance clusterrolebinding: %v", err)
	}
	return nil
}

func (m *Maintenance) createMaintenanceJob(_ context.Context, cronSchedule string) error {
	imageTag := m.svc.Config.Data["imageTag"]
	if imageTag == "" {
		return fmt.Errorf("no imageTag field in composition function configuration")
	}

	sa := maintServiceAccountName
	jobNamespace := m.instanceNamespace
	jobName := "maintenancejob"

	// For helm based services create the job in the control namespace
	if m.helmBasedService {
		jobName = m.resource.GetName() + "-maintenancejob"
		jobNamespace = m.svc.Config.Data["controlNamespace"]
		if jobNamespace == "" {
			return fmt.Errorf("no controlNamespace field in composition function configuration")
		}
		sa = m.svc.Config.Data["maintenanceSA"]
		if sa == "" {
			return fmt.Errorf("no maintenanceSA field in composition function configuration")
		}
	}

	// Build the pod template spec for maintenance jobs
	podTemplateSpec := m.buildMaintenancePodTemplateSpec(imageTag, sa)

	job := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: jobNamespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   cronSchedule,
			SuccessfulJobsHistoryLimit: ptr.To(int32(1)),
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: podTemplateSpec,
				},
			},
		},
	}

	kubeOpts := append([]runtime.KubeObjectOption{runtime.KubeOptionAllowDeletion}, m.extraKubeOptions...)

	return m.svc.SetDesiredKubeObject(job, m.resource.GetName()+"-maintenancejob", kubeOpts...)
}

// buildMaintenancePodTemplateSpec creates the pod template spec for maintenance jobs
// This is shared between the CronJob and the initial one-time Job
func (m *Maintenance) buildMaintenancePodTemplateSpec(imageTag, serviceAccount string) corev1.PodTemplateSpec {
	envVars := []corev1.EnvVar{
		{
			Name:  "INSTANCE_NAMESPACE",
			Value: m.instanceNamespace,
		},
		{
			Name:  "CLAIM_NAME",
			Value: m.resource.GetLabels()["crossplane.io/claim-name"],
		},
		{
			Name:  "COMPOSITE_NAME",
			Value: m.resource.GetName(),
		},
		{
			Name:  "CLAIM_NAMESPACE",
			Value: m.resource.GetLabels()["crossplane.io/claim-namespace"],
		},
		{
			Name:  "OWNER_GROUP",
			Value: m.svc.Config.Data["ownerGroup"],
		},
		{
			Name:  "OWNER_VERSION",
			Value: m.svc.Config.Data["ownerVersion"],
		},
		{
			Name:  "OWNER_KIND",
			Value: m.svc.Config.Data["ownerKind"],
		},
		{
			Name:  "SERVICE_ID",
			Value: m.svc.Config.Data["serviceID"],
		},
		{
			Name:  "RELEASE_MANAGEMENT_ENABLED",
			Value: m.svc.Config.Data["releaseManagementEnabled"],
		},
		{
			Name:  "MAINTENANCE_URL",
			Value: m.svc.Config.Data["maintenanceURL"],
		},
		{
			Name:  "MINIMUM_REVISION_AGE",
			Value: m.svc.Config.Data["minimumRevisionAge"],
		},
		{
			Name:  "PIN_IMAGE_TAG",
			Value: m.schedule.PinImageTag,
		},
		{
			Name:  "DISABLE_APPCAT_RELEASE",
			Value: strconv.FormatBool(m.schedule.DisableAppcatRelease),
		},
	}

	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			ServiceAccountName: serviceAccount,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "maintenancejob",
					Image: "ghcr.io/vshn/appcat:" + imageTag,
					Env:   append(envVars, m.extraEnvs...),
					Args: []string{
						"maintenance",
						"--service",
						m.service,
					},
				},
			},
		},
	}
}
func (m *Maintenance) createMaintenanceClusterRoleBinding(_ context.Context) error {
	name := m.svc.Config.Data["additionalMaintenanceClusterRole"]
	if name == "" {
		return fmt.Errorf("no additionalMaintenanceClusterRole field in composition function configuration")
	}
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.additionalClusterRoleBinding,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      maintServiceAccountName,
				Namespace: m.instanceNamespace,
			},
		},
	}

	return m.svc.SetDesiredKubeObject(clusterRoleBinding, m.resource.GetName()+"-maintenance-clusterrolebinding", m.extraKubeOptions...)
}

func (m *Maintenance) createMaintenanceRoleBinding(_ context.Context) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.mainRole,
			Namespace: m.instanceNamespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     m.mainRole,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: maintServiceAccountName,
			},
		},
	}

	return m.svc.SetDesiredKubeObject(roleBinding, m.resource.GetName()+"-maintenance-rolebinding", m.extraKubeOptions...)
}

func (m *Maintenance) createMaintenanceRole(_ context.Context) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.mainRole,
			Namespace: m.instanceNamespace,
		},
		Rules: m.policyRules,
	}

	return m.svc.SetDesiredKubeObject(role, m.resource.GetName()+"-maintenance-role", m.extraKubeOptions...)
}

func (m *Maintenance) createMaintenanceServiceAccount(_ context.Context) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      maintServiceAccountName,
			Namespace: m.instanceNamespace,
		},
	}

	return m.svc.SetDesiredKubeObject(sa, m.resource.GetName()+"-maintenance-serviceaccount", m.extraKubeOptions...)
}

func (m *Maintenance) parseCron() (string, error) {

	if m.schedule.DayOfWeek == "" || m.schedule.TimeOfDay == "" {
		return "", nil
	}

	cronDayOfWeek := dayOfWeekMap[m.schedule.DayOfWeek]

	r := regexp.MustCompile(`(\d+):(\d+):.*`)
	timeSlice := r.FindStringSubmatch(string(m.schedule.TimeOfDay))

	if len(timeSlice) == 0 {
		return "", fmt.Errorf("not a valid time string %s", m.schedule.TimeOfDay)
	}

	return fmt.Sprintf("%s %s * * %d", timeSlice[2], timeSlice[1], cronDayOfWeek), nil
}

// helper function to return the name of the initial maintenance job
func (m *Maintenance) getInitialMaintenanceJobName() string {
	return m.resource.GetName() + "-initial-maintenance"
}

// createInitialMaintenanceJob creates a one-time Job that runs immediately on instance creation
func (m *Maintenance) createInitialMaintenanceJob(_ context.Context) error {
	imageTag := m.svc.Config.Data["imageTag"]
	if imageTag == "" {
		return fmt.Errorf("no imageTag field in composition function configuration")
	}

	sa := maintServiceAccountName
	jobNamespace := m.instanceNamespace

	// For helm based services create the job in the control namespace
	if m.helmBasedService {
		jobNamespace = m.svc.Config.Data["controlNamespace"]
		if jobNamespace == "" {
			return fmt.Errorf("no controlNamespace field in composition function configuration")
		}
		sa = m.svc.Config.Data["maintenanceSA"]
		if sa == "" {
			return fmt.Errorf("no maintenanceSA field in composition function configuration")
		}
	}

	podTemplateSpec := m.buildMaintenancePodTemplateSpec(imageTag, sa)

	// Create a one-time Job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.getInitialMaintenanceJobName(),
			Namespace: jobNamespace,
		},
		Spec: batchv1.JobSpec{
			Template: podTemplateSpec,
		},
	}

	kubeOpts := append([]runtime.KubeObjectOption{runtime.KubeOptionAllowDeletion}, m.extraKubeOptions...)

	return m.svc.SetDesiredKubeObject(job, m.getInitialMaintenanceJobName(), kubeOpts...)
}

// SetReleaseVersion sets the version from the claim if it's a new instance otherwise it is managed by maintenance function.
// It will return the concrete observed version as well.
// If the desired values contain a higher version than either the observed or the comp version, it will take precedence.
//
// If pinImageTag is set (non-empty):
//   - The exact pinned tag is used unconditionally
//   - Downgrades ARE allowed: the customer takes full responsibility
//   - No version comparison is performed
func SetReleaseVersion(ctx context.Context, version string, desiredValues map[string]interface{}, observedValues map[string]interface{}, fields []string, pinImageTag string) (string, error) {
	l := controllerruntime.LoggerFrom(ctx)

	// If an image tag is pinned, use it unconditionally (allow downgrades)
	if pinImageTag != "" {
		l.Info("Using pinned image tag", "pinnedTag", pinImageTag)
		return pinImageTag, unstructured.SetNestedField(desiredValues, pinImageTag, fields...)
	}

	observedValueVersion, _, err := unstructured.NestedString(observedValues, fields...)
	if err != nil {
		return "", fmt.Errorf("cannot get image tag from values in release: %v", err)
	}

	compVersion, err := semver.ParseTolerant(version)
	if err != nil {
		l.Info("failed to parse desired service version", "version", version)
		return "", fmt.Errorf("invalid service version %q", version)
	}

	desiredValueVersion, _, err := unstructured.NestedString(desiredValues, fields...)
	if err != nil {
		return "", fmt.Errorf("cannot get image tag from values in release: %v", err)
	}

	desiredVersion, err := semver.ParseTolerant(desiredValueVersion)
	if err != nil {
		l.Info("failed to parse desired values version, comparing composite and observed", "version", desiredValueVersion)

		observedVersion, err := semver.ParseTolerant(observedValueVersion)
		if err != nil {
			l.Info("failed to parse observed service version, using composite version", "version", version)
			return version, unstructured.SetNestedField(desiredValues, version, fields...)
		}

		// Return the higher of composite or observed version
		if compVersion.GTE(observedVersion) {
			return version, unstructured.SetNestedField(desiredValues, version, fields...)
		}
		return observedValueVersion, unstructured.SetNestedField(desiredValues, observedValueVersion, fields...)
	}

	observedVersion, err := semver.ParseTolerant(observedValueVersion)
	if err != nil {
		l.Info("failed to parse observed service version, using composite version", "version", version)
		// If the observed version is not parsable, e.g. if it's empty, update to the desired version
		return version, unstructured.SetNestedField(desiredValues, version, fields...)
	}

	if desiredVersion.GTE(observedVersion) {
		return desiredValueVersion, nil
	}

	if observedVersion.GTE(compVersion) {
		// In case the observed tag is valid and greater than the desired version, keep the observed version
		return observedValueVersion, unstructured.SetNestedField(desiredValues, observedValueVersion, fields...)
	}

	// In case the observed tag is smaller than the desired version, then set the version from the claim
	return version, unstructured.SetNestedField(desiredValues, version, fields...)
}
