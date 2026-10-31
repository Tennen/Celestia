package accounts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func localFixture(t *testing.T, instance string, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	root := t.TempDir()
	dir := filepath.Join(root, ".local/admin")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"host": "0.0.0.0", "port": port, "instance": instance})
	if err := os.WriteFile(filepath.Join(dir, "service.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "access-key"), []byte("test-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLocalClientUsesIdentityLoginAndNarrowExecutionPUT(t *testing.T) {
	id := testAccount(1).ID
	logins, writes := 0, 0
	root := localFixture(t, "instance", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Origin") != "http://"+r.Host {
			t.Error("missing local origin")
		}
		if r.URL.Path == "/api/session" {
			_, _ = w.Write([]byte(`{"authenticated":false,"noAuth":false,"serviceInstance":"instance"}`))
			return
		}
		if r.URL.Path == "/api/login" {
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input["password"] != "test-secret" {
				t.Error("wrong login")
			}
			logins++
			http.SetCookie(w, &http.Cookie{Name: "garden_session", Value: "session", Path: "/"})
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		if c, e := r.Cookie("garden_session"); e != nil || c.Value != "session" {
			t.Error("missing session cookie")
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/state" {
			_ = json.NewEncoder(w).Encode(map[string]any{"accounts": []any{map[string]any{"id": id, "roleName": "角色名", "schedule": map[string]bool{"enabled": true}}}})
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/api/accounts/"+id+"/execution" {
			var input map[string]any
			_ = json.NewDecoder(r.Body).Decode(&input)
			if len(input) != 1 || input["enabled"] != false {
				t.Error("account configuration replaced", input)
			}
			writes++
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "roleName": "角色名", "schedule": map[string]bool{"enabled": false}})
			return
		}
		t.Error("unexpected endpoint", r.Method, r.URL.Path)
		w.WriteHeader(404)
	})
	client := LocalClient{}
	list, err := client.Accounts(context.Background(), root)
	if err != nil || len(list) != 1 || list[0].Name != "角色名" || !list[0].AutomationEnabled {
		t.Fatal(list, err)
	}
	a, err := client.SetExecution(context.Background(), root, id, false)
	if err != nil || a.AutomationEnabled || logins != 2 || writes != 1 {
		t.Fatal(a, err, logins, writes)
	}
}

func TestLocalClientRejectsWrongServiceAndUnconfirmedResult(t *testing.T) {
	root := localFixture(t, "expected", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session" {
			t.Error("mismatched service reached protected API")
		}
		_, _ = w.Write([]byte(`{"noAuth":true,"serviceInstance":"other"}`))
	})
	if _, err := (LocalClient{}).Accounts(context.Background(), root); err == nil {
		t.Fatal("service identity unchecked")
	}
	root = localFixture(t, "expected", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/session" {
			_, _ = w.Write([]byte(`{"noAuth":true,"serviceInstance":"expected"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": testAccount(1).ID, "roleName": "角色名", "schedule": map[string]bool{"enabled": true}})
	})
	if _, err := (LocalClient{}).SetExecution(context.Background(), root, testAccount(1).ID, false); err == nil {
		t.Fatal("unconfirmed execution response accepted")
	}
}
