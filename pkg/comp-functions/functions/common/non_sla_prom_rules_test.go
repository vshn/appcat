package common

import (
	"regexp"
	"testing"
)

func TestMain(t *testing.T) {
	checkValidNamespaceRegex := regexp.MustCompile(`[a-z]*-[a-z]*-\(\.\+\)\-\.\+`)
	goodTestCases := []string{
		"vshn-postgresql-development-app1",
		"vshn-postgresql-prod-app2",
		"vshn-minio-main-cluster-prod",
		"vshn-mariadb-prd",
		"vshn-kafka-with-very-long-but-valid-name-including-many-separators-because-we-can",
	}

	brokenCases := []string{
		"",
		"vshn-postgresql",
		"vshn-postgresql-",
		"a-b-a",
		"vshnpostgresqlnoseparator",
		"vshn-redi1s-sadfasd",
	}

	for _, val := range goodTestCases {
		regex, _, err := getInstanceNamespaceRegex(val)
		if err != nil && !checkValidNamespaceRegex.MatchString(regex) {
			t.Logf("Failed goodTestCases test case for: %s, with error: %v", val, err)
			t.FailNow()
		}
	}

	for _, val := range brokenCases {
		regex, d1, err := getInstanceNamespaceRegex(val)
		if err == nil && checkValidNamespaceRegex.MatchString(regex) {
			t.Logf("Failed brokenCases test case for: %s, with error: %v", val, err)
			t.Log(regex, d1)
			t.FailNow()
		}
	}
}
