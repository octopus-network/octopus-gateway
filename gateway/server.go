package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	InitLogger()

	// Route URL: http://gateway-api/route/{chain_id}/{project_id}
	routeChecker := "http://gateway-api/route"
	if value, ok := os.LookupEnv("GATEWAY_API_ROUTE_URL"); ok {
		routeChecker = value
	}

	err := http.ListenAndServe(":80", NewRouter(routeChecker))
	if err != nil {
		log.Fatalln(err)
	}
}
