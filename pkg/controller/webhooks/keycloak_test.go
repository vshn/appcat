package webhooks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func Test_validateCustomFilePaths(t *testing.T) {
	t.Log("Expect error: Empty source")
	assert.Error(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "",
				Destination: "file",
			},
		},
	))

	t.Log("Expect error: Empty destination")
	assert.Error(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "file",
				Destination: "",
			},
		},
	))

	t.Log("Expect error: Root folders")
	for _, folder := range keycloakRootFolders {
		t.Logf("Testing: %s", folder)
		assert.Error(t, validateCustomFilePaths(
			[]vshnv1.VSHNKeycloakCustomFile{
				{
					Source:      "file",
					Destination: fmt.Sprintf("%s/file", folder),
				},
			},
		))
		assert.Error(t, validateCustomFilePaths(
			[]vshnv1.VSHNKeycloakCustomFile{
				{
					Source:      "folder",
					Destination: folder,
				},
			},
		))
	}

	t.Log("Expect error: Path traversal")
	assert.Error(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "passwd",
				Destination: "../../etc/passwd",
			},
		},
	))

	t.Log("Expect no error: Valid destination")
	assert.NoError(t, validateCustomFilePaths(
		[]vshnv1.VSHNKeycloakCustomFile{
			{
				Source:      "blacklist.txt",
				Destination: "data/password-blacklists/blacklist.txt",
			},
		},
	))
}
