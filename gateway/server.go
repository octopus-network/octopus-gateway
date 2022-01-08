package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	limiter := "http://gateway-api/api/limiter"
	if value, ok := os.LookupEnv("GATEWAY_LIMITER_URL"); ok {
		limiter = value
	}

	err := http.ListenAndServe(":8888", NewRouter(limiter))
	if err != nil {
		log.Fatalln(err)
	}
}
