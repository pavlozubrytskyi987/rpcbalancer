package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	target, err := url.Parse("http://localhost:9001")
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	log.Println("rpcbalancer listening on :8080 ->", target)
	log.Fatal(http.ListenAndServe(":8080", proxy))
}
