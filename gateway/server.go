package main

import (
	"log"
	"net"
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

	go func() {
		log.Println("Starting HTTP server on port 80...")
		err := http.ListenAndServe(":80", NewRouter(routeChecker))
		if err != nil {
			log.Fatalln(err)
		}
	}()

	grpcListener, err := net.Listen("tcp", ":81")
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Starting gRPC server on port 81...")
	grpcServer := buildGrpcProxyServer(routeChecker)
	if err = grpcServer.Serve(grpcListener); err != nil {
		log.Fatalln(err)
	}
}
