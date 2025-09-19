package webhooks

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	appcatruntime "github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=delete,path=/validate-appcat-vshn-io-v1-xobjectbucket,mutating=false,failurePolicy=fail,groups=appcat.vshn.io,resources=xobjectbuckets,versions=v1,name=xobjectbuckets.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

const (
	defaultRetentionDays = 6 // Default KeepDaily value from K8upRetentionPolicy
)

var _ webhook.CustomValidator = &XObjectbucketDeletionProtectionHandler{}

// XObjectbucketDeletionProtectionHandler
type XObjectbucketDeletionProtectionHandler struct {
	client             client.Client
	controlPlaneClient client.Client
	log                logr.Logger
}

// SetupXObjectbucketCDeletionProtectionHandlerWithManager registers the validation webhook with the manager.
func SetupXObjectbucketCDeletionProtectionHandlerWithManager(mgr ctrl.Manager) error {

	return ctrl.NewWebhookManagedBy(mgr).
		For(&appcatv1.XObjectBucket{}).
		WithValidator(&XObjectbucketDeletionProtectionHandler{
			client: mgr.GetClient(),
			log:    mgr.GetLogger().WithName("webhook").WithName("objectbucket"),
		}).
		Complete()
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *XObjectbucketDeletionProtectionHandler) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (p *XObjectbucketDeletionProtectionHandler) ValidateUpdate(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
	// NOOP for now
	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (p *XObjectbucketDeletionProtectionHandler) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {

	bucket, ok := obj.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object is not valid")
	}

	creationTimestamp := bucket.GetCreationTimestamp()
	allowedDeletionTime := creationTimestamp.Add(61 * time.Minute)

	now := time.Now()
	age := now.Sub(creationTimestamp.Time)

	if age < 61*time.Minute {
		return nil, fmt.Errorf("XObjectBucket is too young to be deleted, need to wait another %.1f minutes to ensure correct billing", allowedDeletionTime.Sub(now).Minutes())
	}

	l := p.log.WithValues("object", bucket.GetName(), "namespace", bucket.GetNamespace(), "GVK", bucket.GetObjectKind().GroupVersionKind().String())

	compInfo, err := checkManagedObject(ctx, bucket, p.client, p.controlPlaneClient, l)
	if err != nil {
		return nil, err
	}

	// Check for backup disabled timestamp label for retention-based deletion
	if backupDisabledTimestamp, hasDisabledLabel := bucket.GetLabels()[backup.BackupDisabledTimestampLabel]; hasDisabledLabel {
		l.Info("Found backup disabled timestamp label, checking retention period", "timestamp", backupDisabledTimestamp)

		// Parse the timestamp
		timestampUnix, err := strconv.ParseInt(backupDisabledTimestamp, 10, 64)
		if err != nil {
			l.Error(err, "Failed to parse backup disabled timestamp, allowing deletion")
		} else {
			backupDisabledTime := time.Unix(timestampUnix, 0)

			// Get retention period - try to get it from the composite, otherwise use default
			retentionDays := p.getRetentionDays(ctx, bucket, compInfo, l)
			retentionPeriod := time.Duration(retentionDays) * 24 * time.Hour

			allowedDeletionTime := backupDisabledTime.Add(retentionPeriod)

			if now.Before(allowedDeletionTime) {
				timeRemaining := allowedDeletionTime.Sub(now)
				l.Info("Backup disabled bucket still in retention period, blocking deletion",
					"retentionDays", retentionDays,
					"backupDisabledTime", backupDisabledTime.Format(time.RFC3339),
					"allowedDeletionTime", allowedDeletionTime.Format(time.RFC3339),
					"timeRemaining", timeRemaining.String())

				return nil, fmt.Errorf("XObjectBucket backup was disabled but retention period has not expired yet. "+
					"Backup was disabled on %s, retention period is %d days. "+
					"Need to wait another %.1f hours before deletion is allowed",
					backupDisabledTime.Format("2006-01-02 15:04:05 UTC"),
					retentionDays,
					timeRemaining.Hours())
			}

			l.Info("Backup disabled bucket retention period expired, allowing deletion",
				"retentionDays", retentionDays,
				"backupDisabledTime", backupDisabledTime.Format(time.RFC3339),
				"now", now.Format(time.RFC3339))

			// Retention period has passed, allow deletion regardless of composite existence
			return nil, nil
		}
	}

	if compInfo.Exists {
		l.Info("Blocking deletion of XObjectBucket", "parent", compInfo.Name)
		message := protectedMessage
		if compInfo.Reason != "" {
			message = compInfo.Reason
		}
		return nil, fmt.Errorf(message, "XObjectBucket", compInfo.Name)
	}

	l.Info("Allowing deletion of XObjectBucket", "parent", compInfo.Name, "age", age.String())

	return nil, nil
}

// getRetentionDays attempts to get the retention policy from the composite.
// If it cannot retrieve it, it returns the default value.
func (p *XObjectbucketDeletionProtectionHandler) getRetentionDays(ctx context.Context, bucket client.Object, compInfo compositeInfo, l logr.Logger) int {
	if !compInfo.Exists {
		l.V(1).Info("Composite doesn't exist, using default retention", "defaultRetentionDays", defaultRetentionDays)
		return defaultRetentionDays
	}

	// Try to get the composite object to extract retention policy using owner labels
	composite, err := p.getCompositeByName(ctx, bucket, l)
	if err != nil {
		l.Error(err, "Failed to get composite for retention policy, using default", "defaultRetentionDays", defaultRetentionDays)
		return defaultRetentionDays
	}

	if composite == nil {
		l.V(1).Info("Could not determine composite type, using default retention", "defaultRetentionDays", defaultRetentionDays)
		return defaultRetentionDays
	}

	// Extract retention policy based on composite type
	if infoGetter, ok := composite.(common.InfoGetter); ok {
		retention := infoGetter.GetBackupRetention()
		if retention.KeepDaily > 0 {
			l.Info("Using retention policy from composite", "compositeName", compInfo.Name, "retentionDays", retention.KeepDaily)
			return retention.KeepDaily
		}
	}

	l.V(1).Info("Could not get retention policy from composite, using default", "defaultRetentionDays", defaultRetentionDays)
	return defaultRetentionDays
}

// getCompositeByName tries to get the composite object using owner labels from the bucket
func (p *XObjectbucketDeletionProtectionHandler) getCompositeByName(ctx context.Context, bucket client.Object, l logr.Logger) (client.Object, error) {
	// Use the owner labels from the bucket to determine the composite type
	ownerName, ok := bucket.GetLabels()[appcatruntime.OwnerCompositeAnnotation]
	if !ok || ownerName == "" {
		return nil, fmt.Errorf("owner composite label not found on bucket")
	}

	ownerKind, ok := bucket.GetLabels()[appcatruntime.OwnerKindAnnotation]
	if !ok || ownerKind == "" {
		return nil, fmt.Errorf("owner kind label not found on bucket")
	}

	ownerVersion, ok := bucket.GetLabels()[appcatruntime.OwnerVersionAnnotation]
	if !ok || ownerVersion == "" {
		return nil, fmt.Errorf("owner version label not found on bucket")
	}

	ownerGroup, ok := bucket.GetLabels()[appcatruntime.OwnerGroupAnnotation]
	if !ok || ownerGroup == "" {
		return nil, fmt.Errorf("owner group label not found on bucket")
	}

	gvk := schema.GroupVersionKind{
		Kind:    ownerKind,
		Version: ownerVersion,
		Group:   ownerGroup,
	}

	kubeClient := p.controlPlaneClient
	if kubeClient == nil {
		kubeClient = p.client
	}

	rcomp, err := kubeClient.Scheme().New(gvk)
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			return nil, fmt.Errorf("composite type %s not registered in scheme", gvk.String())
		}
		return nil, fmt.Errorf("failed to create composite object: %w", err)
	}

	comp, ok := rcomp.(client.Object)
	if !ok {
		return nil, fmt.Errorf("object is not a valid client object: %s", ownerName)
	}

	err = kubeClient.Get(ctx, client.ObjectKey{Name: ownerName}, comp)
	if err != nil {
		return nil, fmt.Errorf("failed to get composite %s: %w", ownerName, err)
	}

	l.V(1).Info("Found composite using owner labels", "name", ownerName, "type", gvk.String())
	return comp, nil
}
