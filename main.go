package main

import (
	"log"
	"net/http"
)

func main() {
	log.Fatal(http.ListenAndServe(config.Port, Router()))
}
