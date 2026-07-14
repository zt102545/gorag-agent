package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestWeComAPIClient_ListPermitUsersCachesAccessToken(t *testing.T) {
	var tokenCalls atomic.Int32
	var memberCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			tokenCalls.Add(1)
			if r.Method != http.MethodGet || r.URL.Query().Get("corpid") != "ww-test" || r.URL.Query().Get("corpsecret") != "secret" {
				t.Errorf("unexpected token request: %s %s", r.Method, r.URL.String())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "access_token": "token-1", "expires_in": 7200})
		case "/cgi-bin/msgaudit/get_permit_user_list":
			memberCalls.Add(1)
			if r.Method != http.MethodPost || r.URL.Query().Get("access_token") != "token-1" {
				t.Errorf("unexpected member request: %s %s", r.Method, r.URL.String())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok", "ids": []string{"user-b", "user-a", "user-a"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newWeComAPIClient()
	client.baseURL = server.URL
	for i := 0; i < 2; i++ {
		users, err := client.ListPermitUsers(context.Background(), "ww-test", "secret")
		if err != nil {
			t.Fatal(err)
		}
		if len(users) != 2 || users[0] != "user-a" || users[1] != "user-b" {
			t.Fatalf("unexpected users: %#v", users)
		}
	}
	if tokenCalls.Load() != 1 || memberCalls.Load() != 1 {
		t.Fatalf("calls: token=%d members=%d", tokenCalls.Load(), memberCalls.Load())
	}
}
