package main

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func registerResetPasswordRoutes(se *core.ServeEvent) {
	servePage := func(e *core.RequestEvent) error {
		return e.FileFS(os.DirFS("./pb_public"), "reset-password.html")
	}
	se.Router.GET("/reset-password", servePage)
	se.Router.GET("/reset-password/{$}", servePage)
}
