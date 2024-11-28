package main

import (
	"encoding/json"
	"github.com/akrylysov/algnhsa"
	"net/http"
)

func helloWorldHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")

		response := map[string]string{"message": "Hello, World"}
		json.NewEncoder(w).Encode(response)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getHttpServer() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", helloWorldHandler)

	return mux
}

func main() {
	mux := getHttpServer()
	algnhsa.ListenAndServe(mux, nil)
}
