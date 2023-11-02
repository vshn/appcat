package v1

type AwsDBaaSServiceSpec struct {
	// +kubebuilder:validation:Enum=us-east-2;us-east-1;us-west-1;us-west-2;af-south-1;ap-east-1;ap-south-2;ap-southeast-3;ap-southeast-4;ap-south-1;ap-northeast-3;ap-northeast-2;ap-southeast-1;ap-southeast-2;ap-northeast-1;ca-central-1;eu-central-1;eu-west-1;eu-west-2;eu-south-1;eu-west-3;eu-south-2;eu-north-1;eu-central-2;il-central-1;me-south-1;me-central-1;sa-east-us-east-2;us-east-1;us-west-1;us-west-2;af-south-1;ap-east-1;ap-south-2;ap-southeast-3;ap-southeast-4;ap-south-1;ap-northeast-3;ap-northeast-2;ap-southeast-1;ap-southeast-2;ap-northeast-1;ca-central-1;eu-central-1;eu-west-1;eu-west-2;eu-south-1;eu-west-3;eu-south-2;eu-north-1;eu-central-2;il-central-1;me-south-1;me-central-1;sa-east-11
	// +kubebuilder:default="eu-central-1"

	// Region is the datacenter identifier in which the instance runs in.
	Region string `json:"region,omitempty"`
}

type AwsDBaaSMaintenanceScheduleSpec struct {
	// +kubebuilder:default="Wed:00:00-Wed:03:00"

	// The window to perform maintenance in.
	// Syntax: "ddd:hh24:mi-ddd:hh24:mi". Eg: "Mon:00:00-Mon:03:00". See RDS
	// Maintenance Window
	// docs
	// for more information.
	MaintenanceWindow string `json:"maintenanceWindow,omitempty"`
}

type AwsDBaaSSizeSpec struct {
	// +kubebuilder:default="db.t3.micro"

	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`

	// +kubebuilder:default=10

	// StorageSize is the size of the storage in GB.
	StorageSize int `json:"storageSize,omitempty"`

	// +kubebuilder:default="gp2"

	// StorageType is the storage type to use.
	StorageType string `json:"storageType,omitempty"`
}

type AwsDBaaSBackupSpec struct {
	// +kubebuilder:validation:Pattern="^([0-1]?[0-9]|2[0-3]):([0-5][0-9])-([0-1]?[0-9]|2[0-3]):([0-5][0-9])$"
	// +kubebuilder:default="21:30-22:00"

	// BackupWindow for doing daily backups, in UTC.
	// Format: "hh:mm:ss".
	BackupWindow string `json:"backupWindow,omitempty"`

	// +kubebuilder:default=7
	// RetentionPeriod is the number of days to retain backups for.
	RetentionPeriod int `json:"retentionPeriod,omitempty"`
}
