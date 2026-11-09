package editorial

import (
	"context"
	"net/http"
	"net/http/httptest"
	"server/internal/ojs"
	"testing"
	"time"
)

func TestOJSChiefEditorSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/index.php/jserd/login/signIn":
			http.SetCookie(resp, &http.Cookie{Name: "OJSSID", Value: "session", Path: "/"})
			_, _ = resp.Write([]byte("ok"))
		case "/index.php/jserd/api/v1/users":
			resp.Header().Set("Content-Type", "application/json")
			_, _ = resp.Write([]byte(`{"items":[{"id":1,"fullName":"Editora","email":"chief@example.test","groups":[{"id":"chief","name":"Editoras-chefes"},{"id":"author","name":"Autores"}]}],"itemsMax":1}`))
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
	}))
	defer server.Close()
	client, err := ojs.NewClient(server.URL+"/index.php", "jserd", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := (OJSChiefEditorSource{Client: client, Username: "user", Password: "secret", GroupID: testGroupIDChief}).Groups(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || !groups[0].Specific || len(groups[0].Members) != 1 || groups[1].Specific {
		t.Fatalf("groups=%#v", groups)
	}
}
