package main

import (
	"os"
	"server/internal/state"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// serveStaticFiles serves the client build from the public directory.
func serveStaticFiles(se *core.ServeEvent) error {
	registerResetPasswordRoutes(se)
	se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))
	return se.Next()
}

func registerImmutableHooksOnServe(app core.App) {
	app.OnServe().BindFunc(func(event *core.ServeEvent) error {
		state.RegisterImmutableHooks(event.App)
		return event.Next()
	})
}
