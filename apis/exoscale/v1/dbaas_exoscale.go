package v1

type ExoscaleDBaaSServiceSpec struct {
	// +kubebuilder:validation:Enum=ch-gva-2;ch-dk-2;de-fra-1;de-muc-1;at-vie-1;bg-sof-1
	// +kubebuilder:default="ch-gva-2"

	// Zone is the datacenter identifier in which the instance runs in.
	Zone string `json:"zone,omitempty"`
}

type ExoscaleDBaaSMaintenanceScheduleSpec struct {
	// +kubebuilder:validation:Enum=monday;tuesday;wednesday;thursday;friday;saturday;sunday;never
	// +kubebuilder:default="tuesday"

	// DayOfWeek specifies at which weekday the maintenance is held place.
	// Allowed values are [monday, tuesday, wednesday, thursday, friday, saturday, sunday, never]
	DayOfWeek string `json:"dayOfWeek,omitempty"`

	// +kubebuilder:validation:Pattern="^([0-1]?[0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9])$"
	// +kubebuilder:default="22:30:00"

	// TimeOfDay for installing updates in UTC.
	// Format: "hh:mm:ss".
	TimeOfDay string `json:"timeOfDay,omitempty"`
}

type ExoscaleDBaaSSizeSpec struct {
	// +kubebuilder:default="startup-4"

	// Plan is the name of the resource plan that defines the compute resources.
	Plan string `json:"plan,omitempty"`
}

type ExoscaleDBaaSNetworkSpec struct {
	// +kubebuilder:default={"0.0.0.0/0"}

	// IPFilter is a list of allowed IPv4 CIDR ranges that can access the service.
	// If no IP Filter is set, you may not be able to reach the service.
	// A value of `0.0.0.0/0` will open the service to all addresses on the public internet.
	IPFilter []string `json:"ipFilter,omitempty"`
}

type ExoscaleDBaaSBackupSpec struct {
	// +kubebuilder:validation:Pattern="^([0-1]?[0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9])$"
	// +kubebuilder:default="21:30:00"

	// TimeOfDay for doing daily backups, in UTC.
	// Format: "hh:mm:ss".
	TimeOfDay string `json:"timeOfDay,omitempty"`
}
