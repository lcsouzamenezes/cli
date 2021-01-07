package token

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/loophole/cli/internal/pkg/cache"
	"github.com/loophole/cli/internal/pkg/communication"
	tm "github.com/loophole/cli/internal/pkg/token/models"
	"github.com/rs/zerolog/log"
)

const (
	deviceCodeURL = "https://loophole.eu.auth0.com/oauth/device/code"
	tokenURL      = "https://loophole.eu.auth0.com/oauth/token"
	clientID      = "9ocnSAnfJSb6C52waL8xcPidCkRhUwBs"
	scope         = "openid offline_access"
	audience      = "https://api.loophole.cloud"
)

func IsTokenSaved() bool {
	tokensLocation := cache.GetLocalStorageFile("tokens.json", "")

	if _, err := os.Stat(tokensLocation); os.IsNotExist(err) {
		return false
	} else if err != nil {
		communication.LogWarnErr("There was a problem reading tokens file", err)
		return false
	}
	return true
}

func SaveToken(token *tm.TokenSpec) error {
	tokensLocation := cache.GetLocalStorageFile("tokens.json", "")

	tokenBytes, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("There was a problem encoding tokens: %v", err)
	}
	err = ioutil.WriteFile(tokensLocation, tokenBytes, 0644)
	if err != nil {
		return fmt.Errorf("There was a problem writing tokens file: %v", err)
	}
	return nil
}

func RegisterDevice() (*tm.DeviceCodeSpec, error) {
	payload := strings.NewReader(fmt.Sprintf("client_id=%s&scope=%s&audience=%s", url.QueryEscape(clientID), url.QueryEscape(scope), url.QueryEscape(audience)))

	req, err := http.NewRequest("POST", deviceCodeURL, payload)
	if err != nil {
		return nil, fmt.Errorf("There was a problem creating HTTP POST request for device code")
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("There was a problem executing request for device code")
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("There was a problem reading device token response body")
	}

	var jsonResponseBody tm.DeviceCodeSpec
	err = json.Unmarshal(body, &jsonResponseBody)
	if err != nil {
		return nil, fmt.Errorf("There was a problem decoding device token response body")
	}
	return &jsonResponseBody, nil
}

func PollForToken(deviceCode string, interval int, quitChannel <-chan bool) (*tm.TokenSpec, error) {
	grantType := "urn:ietf:params:oauth:grant-type:device_code"

	pollingInterval := time.Duration(interval) * time.Second
	log.Debug().
		Dur("interval", pollingInterval).
		Str("unit", "second").
		Msg("Polling with interval")

	for {
		select {
		case <-quitChannel:
			return nil, fmt.Errorf("Login operation aborted")
		default:
			payload := strings.NewReader(
				fmt.Sprintf("grant_type=%s&device_code=%s&client_id=%s",
					url.QueryEscape(grantType),
					url.QueryEscape(deviceCode),
					url.QueryEscape(clientID)))

			req, err := http.NewRequest("POST", tokenURL, payload)
			if err != nil {
				log.Debug().Err(err).Msg("There was a problem creating HTTP POST request for token")
			}
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			time.Sleep(pollingInterval)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Debug().Err(err).Msg("There was a problem executing request for token")
				continue
			}
			defer res.Body.Close()
			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Debug().
					Bytes("body", body).
					Err(err).
					Msg("There was a problem reading token response body")
				continue
			}

			if res.StatusCode > 400 && res.StatusCode < 500 {
				var jsonResponseBody tm.AuthError
				err := json.Unmarshal(body, &jsonResponseBody)
				if err != nil {
					log.Debug().
						Err(err).
						Bytes("body", body).
						Msg("There was a problem decoding token response body")
					continue
				}
				log.Debug().
					Str("error", jsonResponseBody.Error).
					Str("errorDescription", jsonResponseBody.ErrorDescription).
					Msg("Error response")
				if jsonResponseBody.Error == "authorization_pending" || jsonResponseBody.Error == "slow_down" {
					continue
				} else if jsonResponseBody.Error == "expired_token" || jsonResponseBody.Error == "invalid_grand" {
					return nil, fmt.Errorf("The device token expired, please reinitialize the login")
				} else if jsonResponseBody.Error == "access_denied" {
					return nil, fmt.Errorf("The device token got denied, please reinitialize the login")
				}
			} else if res.StatusCode >= 200 && res.StatusCode <= 300 {
				var jsonResponseBody tm.TokenSpec
				err := json.Unmarshal(body, &jsonResponseBody)
				if err != nil {
					log.Debug().Err(err).Msg("There was a problem decoding token response body")
					continue
				}
				return &jsonResponseBody, nil
			} else {
				return nil, fmt.Errorf("Unexpected response from authorization server: %s", body)
			}
		}
	}
}

func RefreshToken() error {
	grantType := "refresh_token"
	token, err := GetRefreshToken()
	if err != nil {
		return err
	}

	payload := strings.NewReader(fmt.Sprintf("grant_type=%s&client_id=%s&refresh_token=%s", url.QueryEscape(grantType), url.QueryEscape(clientID), url.QueryEscape(token)))

	req, _ := http.NewRequest("POST", tokenURL, payload)

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode > 400 && res.StatusCode < 500 {
		var jsonResponseBody tm.AuthError
		err := json.Unmarshal(body, &jsonResponseBody)
		if err != nil {
			return err
		}
		log.Debug().
			Str("error", jsonResponseBody.Error).
			Str("errorDescription", jsonResponseBody.ErrorDescription).
			Msg("Error response")
		if jsonResponseBody.Error == "expired_token" || jsonResponseBody.Error == "invalid_grand" {
			return fmt.Errorf("The device token expired, please reinitialize the login")
		} else if jsonResponseBody.Error == "access_denied" {
			return fmt.Errorf("The device token got denied, please reinitialize the login")
		}
	} else if res.StatusCode >= 200 && res.StatusCode <= 300 {
		var jsonResponseBody tm.TokenSpec
		err := json.Unmarshal(body, &jsonResponseBody)
		if err != nil {
			return err
		}

		jsonResponseBody.RefreshToken = token

		err = SaveToken(&jsonResponseBody)
		if err != nil {
			return err
		}

	} else {
		return fmt.Errorf("Unexpected response from authorization server: %s", body)
	}
	return nil

}

func DeleteTokens() error {
	tokensLocation := cache.GetLocalStorageFile("tokens.json", "")

	err := os.Remove(tokensLocation)
	if err != nil {
		return fmt.Errorf("There was a problem removing tokens file: %v", err)
	}
	return nil
}

func GetAccessToken() (string, error) {
	tokensLocation := cache.GetLocalStorageFile("tokens.json", "")

	tokens, err := ioutil.ReadFile(tokensLocation)
	if err != nil {
		return "", fmt.Errorf("There was a problem reading tokens: %v", err)
	}
	var token tm.TokenSpec
	err = json.Unmarshal(tokens, &token)
	if err != nil {
		return "", fmt.Errorf("There was a problem decoding tokens: %v", err)
	}
	return token.AccessToken, nil
}

func GetRefreshToken() (string, error) {
	tokensLocation := cache.GetLocalStorageFile("tokens.json", "")

	tokens, err := ioutil.ReadFile(tokensLocation)
	if err != nil {
		return "", fmt.Errorf("There was a problem reading tokens: %v", err)
	}
	var token tm.TokenSpec
	err = json.Unmarshal(tokens, &token)
	if err != nil {
		return "", fmt.Errorf("There was a problem decoding tokens: %v", err)
	}
	return token.RefreshToken, nil
}
