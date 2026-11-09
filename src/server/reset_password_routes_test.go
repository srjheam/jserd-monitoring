package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func TestResetPasswordRoutesServeExportWithoutRedirect(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("pb_public", 0o700); err != nil {
		t.Fatal(err)
	}
	const page = "<html>Reset password</html>"
	if err := os.WriteFile("pb_public/reset-password.html", []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	app := serverTestApp(t)
	defer app.Cleanup()
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	registerResetPasswordRoutes(&core.ServeEvent{App: app, Router: router})
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/reset-password", "/reset-password/"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path+"?token=reset-token", nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusOK || response.Body.String() != page {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if response.Header().Get("Location") != "" || request.URL.Query().Get("token") != "reset-token" {
				t.Fatal("reset page must preserve the token without a redirect")
			}
		})
	}
}
