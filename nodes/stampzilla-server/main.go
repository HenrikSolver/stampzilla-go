//go:generate bash -c "go get -u github.com/rakyll/statik && cd web && npm run build && cd .. && statik -src ./web/dist -f"
package main

import (
	"fmt"

	"stampzilla/nodes/stampzilla-server/models"
	"stampzilla/nodes/stampzilla-server/servermain"
	"stampzilla/pkg/build"

	// Statik for the webserver gui
	_ "stampzilla/nodes/stampzilla-server/statik"
)

func main() {

	config := &models.Config{}
	config.MustLoad()

	if config.Version {
		fmt.Println(build.String())
		return
	}

	server := servermain.New(config)
	server.Init()
	server.Run()
}
