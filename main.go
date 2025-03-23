package main

import (
	"github.com/gofiber/fiber/v2"
)

func main() {
	//init template files
	initTemplates()

	//Load nodes and watch for changes.
	loadAndWatchNodesAndAttachments()

	app := fiber.New()
	initRoutes(app)

    certFile:=getArgValue("--cert")
    keyFile:=getArgValue("--key")
    if certFile=="" || keyFile=="" {
        err := app.Listen(":"+getArgValue("--port"))
        if err!=nil{panic(err)}
    } else {
        err := app.ListenTLS(":"+getArgValue("--port"),certFile,keyFile)
        if err!=nil{panic(err)}
    }
}
