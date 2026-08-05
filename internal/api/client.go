package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"jahandco/cli/internal/auth"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type Project struct {
	ID            string                 `json:"id"`
	OwnerID       string                 `json:"ownerId"`
	Name          string                 `json:"name"`
	Scope         *string                `json:"scope"`
	Visibility    Visibility             `json:"visibility"`
	ScopeMetadata map[string]interface{} `json:"scopeMetadata"`
	CreatedAt     string                 `json:"createdAt"`
}

// parseErrorMessage extracts developer-api's {"error": "..."} body into a
// plain string, same as apps/client's own parseErrorMessage -- falls back
// to the raw body for any response that isn't that shape (e.g. an upstream
// proxy's plain-text error page).
func parseErrorMessage(body []byte) string {
	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != "" {
		return parsed.Error
	}
	return string(body)
}

func getDeveloperAPIURL() string {
	url := os.Getenv("JAHANDCO_DEVELOPER_API_URL")
	if url == "" {
		url = "https://api-developer.jahandco.net"
	}
	return url
}

// DeveloperAPIURL exposes the same base URL as getDeveloperAPIURL for
// callers outside this package that need to build their own request --
// cmd/dev.go's dev-session websocket dial, which can't go through request[T]
// since it isn't a JSON request/response call.
func DeveloperAPIURL() string {
	return getDeveloperAPIURL()
}

func request[T any](method string, pathName string, body interface{}) (T, error) {
	var result T
	token, err := auth.GetToken()
	if err != nil {
		return result, err
	}

	url := fmt.Sprintf("%s%s", getDeveloperAPIURL(), pathName)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return result, err
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return result, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return result, nil
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("[jc] developer-api %s %s failed (%d): %s", method, pathName, resp.StatusCode, parseErrorMessage(respBytes))
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return result, err
	}

	return result, nil
}

func CreateProject(name string, visibility Visibility) (*Project, error) {
	payload := map[string]interface{}{
		"name": name,
	}
	if visibility != "" {
		payload["visibility"] = visibility
	}
	return request[*Project]("POST", "/v1/projects", payload)
}

func ListProjects() ([]*Project, error) {
	return request[[]*Project]("GET", "/v1/projects", nil)
}

func DeleteProject(id string) error {
	_, err := request[interface{}]("DELETE", fmt.Sprintf("/v1/projects/%s", id), nil)
	return err
}

func SetProjectScope(id string, scope string) (*Project, error) {
	payload := map[string]string{
		"scope": scope,
	}
	return request[*Project]("PATCH", fmt.Sprintf("/v1/projects/%s/scope", id), payload)
}

func SetProjectVisibility(id string, visibility Visibility) (*Project, error) {
	payload := map[string]string{
		"visibility": string(visibility),
	}
	return request[*Project]("PATCH", fmt.Sprintf("/v1/projects/%s/visibility", id), payload)
}

type Asset struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	CreatedAt   string `json:"createdAt"`
}

// uploadMultipart is request[T]'s sibling for endpoints that take a file
// body -- POST /v1/projects/{id}/assets is currently the only one, so this
// stays specific to that shape rather than a generic multipart builder.
func uploadMultipart[T any](pathName string, fieldName string, filename string, data []byte) (T, error) {
	var result T
	token, err := auth.GetToken()
	if err != nil {
		return result, err
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(fieldName, filename)
	if err != nil {
		return result, err
	}
	if _, err := part.Write(data); err != nil {
		return result, err
	}
	if err := mw.Close(); err != nil {
		return result, err
	}

	url := fmt.Sprintf("%s%s", getDeveloperAPIURL(), pathName)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("[jc] developer-api POST %s failed (%d): %s", pathName, resp.StatusCode, string(respBytes))
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return result, err
	}

	return result, nil
}

// UploadAsset uploads a file into projectID's persistent asset library.
// Returns the created Asset -- its ID is the literal string developers
// hardcode into jc.assets.get(assetId) calls.
func UploadAsset(projectID string, filename string, data []byte) (*Asset, error) {
	return uploadMultipart[*Asset](fmt.Sprintf("/v1/projects/%s/assets", projectID), "file", filename, data)
}

func ListAssets(projectID string) ([]*Asset, error) {
	return request[[]*Asset]("GET", fmt.Sprintf("/v1/projects/%s/assets", projectID), nil)
}

func DeleteAsset(projectID, assetID string) error {
	_, err := request[interface{}]("DELETE", fmt.Sprintf("/v1/projects/%s/assets/%s", projectID, assetID), nil)
	return err
}

func DirectPost(url string, token string, body interface{}) ([]byte, int, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return respBytes, resp.StatusCode, nil
}
