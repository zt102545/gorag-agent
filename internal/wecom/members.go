package wecom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultWeComAPIBaseURL = "https://qyapi.weixin.qq.com"

type permitUserProvider interface {
	ListPermitUsers(ctx context.Context, corpID, secret string) ([]string, error)
}

type weComAPIClient struct {
	client  *http.Client
	baseURL string

	mu        sync.Mutex
	tokenKey  string
	token     string
	expiresAt time.Time

	usersMu        sync.Mutex
	usersKey       string
	users          []string
	usersExpiresAt time.Time
}

func newWeComAPIClient() *weComAPIClient {
	return &weComAPIClient{client: &http.Client{Timeout: 15 * time.Second}, baseURL: defaultWeComAPIBaseURL}
}

type weComAPIError struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func decodeWeComJSON(response *http.Response, target any) error {
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("WeCom API HTTP status %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(target); err != nil {
		return fmt.Errorf("decode WeCom API response: %w", err)
	}
	return nil
}

func requestError(prefix string, err error) error {
	if urlErr, ok := err.(*url.Error); ok {
		return fmt.Errorf("%s: %v", prefix, urlErr.Err)
	}
	return fmt.Errorf("%s: %v", prefix, err)
}

func (c *weComAPIClient) accessToken(ctx context.Context, corpID, secret string, force bool) (string, error) {
	key := corpID + "\x00" + secret
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && c.tokenKey == key && time.Now().Before(c.expiresAt) {
		return c.token, nil
	}
	endpoint, err := url.Parse(strings.TrimRight(c.baseURL, "/") + "/cgi-bin/gettoken")
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("corpid", corpID)
	query.Set("corpsecret", secret)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return "", requestError("request WeCom access token", err)
	}
	var payload struct {
		weComAPIError
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := decodeWeComJSON(response, &payload); err != nil {
		return "", err
	}
	if payload.ErrCode != 0 {
		return "", fmt.Errorf("get WeCom access token: code=%d message=%s", payload.ErrCode, payload.ErrMsg)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("get WeCom access token: empty token")
	}
	cacheFor := time.Duration(payload.ExpiresIn) * time.Second
	if cacheFor <= 0 {
		cacheFor = 2 * time.Hour
	}
	if cacheFor > 5*time.Minute {
		cacheFor -= 5 * time.Minute
	} else {
		cacheFor /= 2
	}
	c.tokenKey = key
	c.token = payload.AccessToken
	c.expiresAt = time.Now().Add(cacheFor)
	return c.token, nil
}

func (c *weComAPIClient) fetchPermitUsers(ctx context.Context, token string) ([]string, int, error) {
	endpoint, err := url.Parse(strings.TrimRight(c.baseURL, "/") + "/cgi-bin/msgaudit/get_permit_user_list")
	if err != nil {
		return nil, 0, err
	}
	query := endpoint.Query()
	query.Set("access_token", token)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, 0, requestError("request WeCom permit user list", err)
	}
	var payload struct {
		weComAPIError
		IDs []string `json:"ids"`
	}
	if err := decodeWeComJSON(response, &payload); err != nil {
		return nil, 0, err
	}
	if payload.ErrCode != 0 {
		return nil, payload.ErrCode, fmt.Errorf("get WeCom permit user list: code=%d message=%s", payload.ErrCode, payload.ErrMsg)
	}
	result := cleanIDs(payload.IDs)
	sort.Strings(result)
	return result, 0, nil
}

func (c *weComAPIClient) ListPermitUsers(ctx context.Context, corpID, secret string) ([]string, error) {
	key := corpID + "\x00" + secret
	c.usersMu.Lock()
	defer c.usersMu.Unlock()
	if c.usersKey == key && time.Now().Before(c.usersExpiresAt) {
		return append([]string(nil), c.users...), nil
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.accessToken(ctx, corpID, secret, attempt > 0)
		if err != nil {
			return nil, err
		}
		users, code, err := c.fetchPermitUsers(ctx, token)
		if err == nil {
			c.usersKey = key
			c.users = append([]string(nil), users...)
			c.usersExpiresAt = time.Now().Add(30 * time.Second)
			return users, nil
		}
		if attempt == 0 && (code == 40014 || code == 42001 || code == 42007) {
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("get WeCom permit user list failed")
}
