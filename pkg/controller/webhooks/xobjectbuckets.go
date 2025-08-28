package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:verbs=delete,path=/validate-appcat-vshn-io-v1-xobjectbucket,mutating=false,failurePolicy=fail,groups=appcat.vshn.io,resources=xobjectbuckets,versions=v1,name=xobjectbuckets.vshn.appcat.vshn.io,sideEffects=None,admissionReviewVersions=v1

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

	if compInfo.Exists {
		l.Info("Blocking deletion of XObjectBucket", "parent", compInfo.Name)
		message := protectedMessage
		if compInfo.Reason != "" {
			message = compInfo.Reason
		}
		return nil, fmt.Errorf(message, "XObjectBucket", compInfo.Name)
	}

	// Check backup retention if composite no longer exists (backup disabled scenario)
	retentionErr := p.checkBackupRetention(ctx, bucket, creationTimestamp.Time, l)
	if retentionErr != nil {
		return nil, retentionErr
	}

	l.Info("Allowing deletion of XObjectBucket", "parent", compInfo.Name, "age", age.String())

	return nil, nil
}

// checkBackupRetention checks if a backup bucket should be retained based on backup retention policies.
// This is called when the parent composite no longer exists (backup disabled scenario).
func (p *XObjectbucketDeletionProtectionHandler) checkBackupRetention(ctx context.Context, bucket client.Object, creationTimestamp time.Time, l logr.Logger) error {
	// Only check retention for backup buckets
	bucketName := bucket.GetName()
	if !isBackupBucket(bucketName) {
		return nil // Not a backup bucket, skip retention check
	}

	// Get the composite information from labels
	compositeName, exists := bucket.GetLabels()["appcat.vshn.io/ownercomposite"]
	if !exists {
		l.V(1).Info("No composite owner found, skipping retention check")
		return nil
	}

	ownerKind, exists := bucket.GetLabels()["appcat.vshn.io/ownerkind"]
	if !exists {
		l.V(1).Info("No owner kind found, skipping retention check")
		return nil
	}

	// Try to get the composite to check its backup settings
	// Even if it doesn't exist, we can infer retention from creation time
	retentionPeriod, err := p.getRetentionPeriod(ctx, compositeName, ownerKind, l)
	if err != nil {
		l.Error(err, "Failed to determine retention period, allowing deletion")
		return nil // On error, allow deletion to avoid blocking
	}

	// Calculate if we're still within retention period
	now := time.Now()
	retentionEndTime := creationTimestamp.Add(retentionPeriod)

	if now.Before(retentionEndTime) {
		remainingTime := retentionEndTime.Sub(now)
		l.Info("Backup bucket still within retention period", "remaining", remainingTime.String(), "composite", compositeName)
		return fmt.Errorf("backup bucket is within retention period, need to wait another %.1f minutes before deletion", remainingTime.Minutes())
	}

	l.Info("Backup retention period has passed, allowing deletion", "composite", compositeName, "age", now.Sub(creationTimestamp).String())
	return nil
}

// isBackupBucket checks if the bucket name indicates it's a backup bucket
func isBackupBucket(bucketName string) bool {
	// Backup buckets have "-backup" suffix
	return len(bucketName) > 7 && bucketName[len(bucketName)-7:] == "-backup"
}

// getRetentionPeriod determines the retention period for a backup bucket
func (p *XObjectbucketDeletionProtectionHandler) getRetentionPeriod(ctx context.Context, compositeName, ownerKind string, l logr.Logger) (time.Duration, error) {
	// Default retention period
	defaultRetention := 7 * 24 * time.Hour

	// Try to get retention from the composite if it still exists
	switch ownerKind {
	case "XVSHNMariaDB":
		retention, err := p.getMariaDBRetention(ctx, compositeName, l)
		if err == nil {
			return retention, nil
		}
	case "XVSHNPostgreSQL":
		retention, err := p.getPostgreSQLRetention(ctx, compositeName, l)
		if err == nil {
			return retention, nil
		}
	case "XVSHNRedis":
		retention, err := p.getRedisRetention(ctx, compositeName, l)
		if err == nil {
			return retention, nil
		}
		// Add other service types as needed
	}

	l.V(1).Info("Using default retention period", "period", defaultRetention.String())
	return defaultRetention, nil
}

func (p *XObjectbucketDeletionProtectionHandler) getMariaDBRetention(ctx context.Context, compositeName string, l logr.Logger) (time.Duration, error) {
	mariadb := &vshnv1.XVSHNMariaDB{}
	err := p.client.Get(ctx, client.ObjectKey{Name: compositeName}, mariadb)
	if err != nil {
		return 0, err
	}
	return calculateRetentionDuration(mariadb.Spec.Parameters.Backup.Retention), nil
}

func (p *XObjectbucketDeletionProtectionHandler) getPostgreSQLRetention(ctx context.Context, compositeName string, l logr.Logger) (time.Duration, error) {
	postgresql := &vshnv1.XVSHNPostgreSQL{}
	err := p.client.Get(ctx, client.ObjectKey{Name: compositeName}, postgresql)
	if err != nil {
		return 0, err
	}
	// PostgreSQL uses a simple int for retention (days)
	retentionDays := postgresql.Spec.Parameters.Backup.Retention
	if retentionDays <= 0 {
		retentionDays = 7 // Default 7 days
	}
	return time.Duration(retentionDays) * 24 * time.Hour, nil
}

func (p *XObjectbucketDeletionProtectionHandler) getRedisRetention(ctx context.Context, compositeName string, l logr.Logger) (time.Duration, error) {
	redis := &vshnv1.XVSHNRedis{}
	err := p.client.Get(ctx, client.ObjectKey{Name: compositeName}, redis)
	if err != nil {
		return 0, err
	}
	return calculateRetentionDuration(redis.Spec.Parameters.Backup.Retention), nil
}

// calculateRetentionDuration calculates the longest retention period from a K8upRetentionPolicy
func calculateRetentionDuration(retention vshnv1.K8upRetentionPolicy) time.Duration {
	var maxDuration time.Duration

	if retention.KeepLast > 0 {
		if duration := time.Duration(retention.KeepLast) * 24 * time.Hour; duration > maxDuration {
			maxDuration = duration
		}
	}

	if retention.KeepHourly > 0 {
		if duration := time.Duration(retention.KeepHourly) * time.Hour; duration > maxDuration {
			maxDuration = duration
		}
	}

	if retention.KeepDaily > 0 {
		if duration := time.Duration(retention.KeepDaily) * 24 * time.Hour; duration > maxDuration {
			maxDuration = duration
		}
	}

	if retention.KeepWeekly > 0 {
		if duration := time.Duration(retention.KeepWeekly) * 7 * 24 * time.Hour; duration > maxDuration {
			maxDuration = duration
		}
	}

	if retention.KeepMonthly > 0 {
		if duration := time.Duration(retention.KeepMonthly) * 30 * 24 * time.Hour; duration > maxDuration {
			maxDuration = duration
		}
	}

	if retention.KeepYearly > 0 {
		if duration := time.Duration(retention.KeepYearly) * 365 * 24 * time.Hour; duration > maxDuration {
			maxDuration = duration
		}
	}

	// If no retention policy is set, default to 7 days
	if maxDuration == 0 {
		maxDuration = 7 * 24 * time.Hour
	}

	return maxDuration
}
