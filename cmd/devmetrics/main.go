package main

import (
	"log"
	"net/http"
	"os"

	"github.com/example/devtools-metrics-bridge/internal/devmetrics"
	"github.com/example/devtools-metrics-bridge/internal/infrai"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	client := infrai.New(baseURL, key, nil)
	service := devmetrics.New(client)

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("devtools metrics service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, service.Handler()))
}
