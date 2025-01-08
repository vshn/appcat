package stackgres

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-version"
	"net/http"
	"sort"
)

type loginRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type authToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// PgVersions contains available postgres versions
type PgVersions struct {
	Postgresql []string `json:"postgresql"`
}

// StackgresClient creates a client to connect to Stackgres Operator
type StackgresClient struct {
	username, password, sgNamespace, prefixUrl string
	httpClient                                 *http.Client
	token                                      authToken
}

// New creates a Stackgres client from username, password and namespace where Stackgres is running
func New(username, password, sgNamespace string) (*StackgresClient, error) {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	httpClient := &http.Client{Transport: t}

	auth := loginRequest{Username: username, Password: password}

	byteAuth, err := json.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal login json: %w", err)
	}
	prefixUrl := "https://stackgres-restapi." + sgNamespace + ".svc.cluster.local"
	resp, err := httpClient.Post(prefixUrl+"/stackgres/auth/login", "application/json", bytes.NewBuffer(byteAuth))
	if err != nil {
		return nil, fmt.Errorf("cannot login: %w", err)
	}

	token := &authToken{}
	err = json.NewDecoder(resp.Body).Decode(token)
	if err != nil {
		return nil, fmt.Errorf("cannot decode login token: %w", err)
	}

	return &StackgresClient{
		sgNamespace: sgNamespace,
		username:    username,
		password:    password,
		prefixUrl:   prefixUrl,
		httpClient:  httpClient,
		token:       *token,
	}, nil
}

// GetAvailableVersions fetches all available versions
func (c StackgresClient) GetAvailableVersions() (*PgVersions, error) {
	req, err := http.NewRequest("GET", c.prefixUrl+"/stackgres/version/postgresql", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get list of versions: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+c.token.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error during http request: %w", err)
	}
	versionList := &PgVersions{}

	err = json.NewDecoder(resp.Body).Decode(versionList)
	if err != nil {
		return nil, fmt.Errorf("error during json decoding: %w", err)
	}
	return versionList, nil
}

// GetLatestMinorVersion searching most current minor version
func GetLatestMinorVersion(vers string, versionList *PgVersions) (string, error) {

	if versionList == nil {
		return vers, nil
	}

	current, err := version.NewVersion(vers)
	if err != nil {
		return "", err
	}

	validVersions := make([]*version.Version, 0)
	for _, newVersion := range versionList.Postgresql {
		tmpVersion, err := version.NewVersion(newVersion)
		if err != nil {
			return "", err
		}
		if tmpVersion.Segments()[0] == current.Segments()[0] {
			validVersions = append(validVersions, tmpVersion)
		}
	}

	sort.Sort(sort.Reverse(version.Collection(validVersions)))

	if len(validVersions) != 0 && current.LessThan(validVersions[0]) {
		return validVersions[0].Original(), nil
	}

	return current.Original(), nil
}
