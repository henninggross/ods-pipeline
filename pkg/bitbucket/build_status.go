package bitbucket

import (
	"encoding/json"
	"fmt"
)

const (
	BuildStatusInProgress = "INPROGRESS"
	BuildStatusSuccessful = "SUCCESSFUL"
	BuildStatusFailed     = "FAILED"
)

type BuildStatusCreatePayload struct {
	State       string `json:"state"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// BuildStatusCreate associates a build status with a commit.
// The state, the key and the url are mandatory. The name and description fields are optional.
// All fields (mandatory or optional) are limited to 255 characters, except for the url, which is limited to 450 characters.
// Supported values for the state are SUCCESSFUL, FAILED and INPROGRESS.
// The authenticated user must have LICENSED permission or higher to call this resource.
// https://docs.atlassian.com/bitbucket-server/rest/7.13.0/bitbucket-build-rest.html#idp6
func (c *Client) BuildStatusCreate(gitCommit string, payload BuildStatusCreatePayload) error {
	urlPath := fmt.Sprintf("/rest/build-status/1.0/commits/%s", gitCommit)
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	statusCode, response, err := c.post(urlPath, b)
	if err != nil {
		return fmt.Errorf("request returned error: %w", err)
	}
	if statusCode != 204 {
		return fmt.Errorf("request returned unexpected response code: %d, body: %s", statusCode, string(response))
	}
	return nil
}

type BuildStatus struct {
	State       string `json:"state"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	DateAdded   int64  `json:"dateAdded"`
}

type BuildStatusPage struct {
	Size       int           `json:"size"`
	Limit      int           `json:"limit"`
	IsLastPage bool          `json:"isLastPage"`
	Values     []BuildStatus `json:"values"` // newest build status appears first
	Start      int           `json:"start"`
}

// BuildStatusList gets the build statuses associated with a commit.
// https://docs.atlassian.com/bitbucket-server/rest/7.13.0/bitbucket-build-rest.html#idp8
func (c *Client) BuildStatusList(gitCommit string) (*BuildStatusPage, error) {
	urlPath := fmt.Sprintf("/rest/build-status/1.0/commits/%s", gitCommit)
	statusCode, response, err := c.get(urlPath)
	if err != nil {
		return nil, fmt.Errorf("request returned error: %w", err)
	}

	switch statusCode {
	case 200:
		var buildStatusPage BuildStatusPage
		err = json.Unmarshal(response, &buildStatusPage)
		if err != nil {
			return nil, fmt.Errorf(
				"could not unmarshal response: %w. status code: %d, body: %s", err, statusCode, string(response),
			)
		}
		return &buildStatusPage, nil
	case 401:
		return nil, fmt.Errorf("you are not permitted to get the build status of git commit %s", gitCommit)
	default:
		return nil, fmt.Errorf("unexpected status code %d", statusCode)
	}

}
