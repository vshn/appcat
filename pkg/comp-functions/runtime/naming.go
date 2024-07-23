package runtime

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"

	"github.com/crossplane/function-sdk-go/resource/composed"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultDNSLabelLength = 63
const jobDSNLabelLenth = 52

// escapeK8sNames will figure out what naming scheme applies to the given object
// and tries to find the correct naming scheme for it.
// Then it will escape the name accordingly.
func escapeK8sNames(obj client.Object) {
	kind, _, err := composed.Scheme.ObjectKinds(obj)
	if err != nil {
		obj.SetName(EscapeDNS1123(obj.GetName(), false))
	}

	switch kind[0].Kind {
	case "Pod":
		obj.SetName(EscapeDNS1123Label(obj.GetName(), defaultDNSLabelLength))
	case "Service":
		obj.SetName(EscapeDNS1123Label(obj.GetName(), defaultDNSLabelLength))
	case "Role":
		obj.SetName(EscapeDNS1123(obj.GetName(), true))
	case "ClusterRole":
		obj.SetName(EscapeDNS1123(obj.GetName(), true))
	case "RoleBinding":
		obj.SetName(EscapeDNS1123(obj.GetName(), true))
	case "ClusterRoleBinding":
		obj.SetName(EscapeDNS1123(obj.GetName(), true))
	case "CronJob":
		obj.SetName(EscapeDNS1123Label(obj.GetName(), jobDSNLabelLenth))
	default:
		obj.SetName(EscapeDNS1123(obj.GetName(), false))
	}
}

// EscapeDNS1123Label does the same as escapeDNS1123 but also limit to 63 chars
func EscapeDNS1123Label(name string, length int) string {
	name = EscapeDNS1123(name, false)
	if len(name) > length {
		suffix := hashString(name)
		name = name[:length-5] + suffix
	}
	return name
}

// EscapeDNS1123 will always return a string that conforms to K8s' DNS subdomain naming scheme
// contain no more than 253 characters
// contain only lowercase alphanumeric characters, '-' or '.'
// start with an alphanumeric character
// end with an alphanumeric character
// We also remove any '.' here, so that it can be used as a base function for escaping
// label names as well.
func EscapeDNS1123(name string, allowColons bool) string {
	escapeNameStart := regexp.MustCompile("^[^a-zA-Z0-9]+")
	escapeNameEnd := regexp.MustCompile("[^a-zA-Z0-9]+$")
	escapeExpression := regexp.MustCompile("[^a-zA-Z0-9]+")
	dns1123LabelRegexColons := "^[a-z0-9]([-a-z0-9:]*[a-z0-9])?$"
	dns1123LabelRegexp := regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")

	if allowColons {
		dns1123LabelRegexp = regexp.MustCompile(dns1123LabelRegexColons)
	}

	if dns1123LabelRegexp.MatchString(name) && len(name) <= 253 {
		return name
	}

	escapedName := strings.ToLower(name)

	escapedName = escapeExpression.ReplaceAllString(escapedName, "-")
	escapedName = escapeNameStart.ReplaceAllString(escapedName, "")
	escapedName = escapeNameEnd.ReplaceAllString(escapedName, "")

	if len(escapedName) > 253 {
		suffix := hashString(escapedName)
		escapedName = escapedName[:248] + suffix
	}

	return escapedName
}

// hashString returns the first 4 symbols of the sha1 hash, prefixed with a `-`
func hashString(name string) string {
	hasher := sha1.New()
	hasher.Write([]byte(name))
	return "-" + fmt.Sprintf("%x", hasher.Sum(nil))[:4]
}
