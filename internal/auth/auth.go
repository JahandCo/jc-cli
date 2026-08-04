package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ClerkAuthorizeURL = "https://clerk.jahandco.dev/oauth/authorize"
	ClerkTokenURL     = "https://clerk.jahandco.dev/oauth/token"
	ClerkClientID     = "L8DS4AaZdAcghDFU"
	ClerkScopes       = "email offline_access profile"
	ClerkRedirectURI  = "https://jahandco.dev/cli-callback"
)

type StoredCredentials struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ObtainedAt   int64  `json:"obtainedAt"`
	ExpiresIn    int64  `json:"expiresIn"`
}

func getCredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".jahandco", "credentials.json"), nil
}

func SaveCredentials(creds *StoredCredentials) error {
	path, err := getCredentialsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func LoadCredentials() (*StoredCredentials, error) {
	path, err := getCredentialsPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var creds StoredCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

func IsExpired(creds *StoredCredentials) bool {
	expiresAt := creds.ObtainedAt + (creds.ExpiresIn * 1000)
	now := time.Now().UnixNano() / int64(time.Millisecond)
	// 30s safety margin
	return now >= (expiresAt - 30000)
}

func GetToken() (string, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	if creds == nil {
		return "", errors.New("[jc] not logged in -- run \"jc login\" first")
	}
	if IsExpired(creds) {
		return "", errors.New("[jc] session expired -- run \"jc login\" again")
	}
	return creds.AccessToken, nil
}

func Base64URLEncode(buf []byte) string {
	return base64.RawURLEncoding.EncodeToString(buf)
}

func GeneratePKCE() (verifier string, challenge string, state string, err error) {
	vBytes := make([]byte, 32)
	if _, err := rand.Read(vBytes); err != nil {
		return "", "", "", err
	}
	verifier = Base64URLEncode(vBytes)

	h := sha256.New()
	h.Write([]byte(verifier))
	challenge = Base64URLEncode(h.Sum(nil))

	sBytes := make([]byte, 16)
	if _, err := rand.Read(sBytes); err != nil {
		return "", "", "", err
	}
	state = Base64URLEncode(sBytes)

	return verifier, challenge, state, nil
}

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func ExchangeCode(code string, verifier string) (*StoredCredentials, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", ClerkRedirectURI)
	data.Set("client_id", ClerkClientID)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequest("POST", ClerkTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenRes TokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenRes); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK || tokenRes.AccessToken == "" {
		errMsg := tokenRes.ErrorDescription
		if errMsg == "" {
			errMsg = tokenRes.Error
		}
		if errMsg == "" {
			errMsg = resp.Status
		}
		return nil, fmt.Errorf("token exchange failed: %s", errMsg)
	}

	expiresIn := tokenRes.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	creds := &StoredCredentials{
		AccessToken:  tokenRes.AccessToken,
		RefreshToken: tokenRes.RefreshToken,
		ObtainedAt:   time.Now().UnixNano() / int64(time.Millisecond),
		ExpiresIn:    expiresIn,
	}

	return creds, nil
}
