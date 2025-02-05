package main

import (
	"fmt"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request from %s to %s%s", r.RemoteAddr, r.Host, r.URL.Path)
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintf(w, "{\"ok\": true}")
}

func main() {
	http.HandleFunc("/", handler)
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
