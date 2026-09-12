package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	fieldservice "example.com/fieldservice-error-capture"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := fieldservice.NewClient(key)

	http.HandleFunc("POST /work-order-failures", func(w http.ResponseWriter, r *http.Request) {
		var input fieldservice.WorkOrderFailure
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		followUp, err := fieldservice.CaptureWorkOrderFailure(r.Context(), client, input)
		if err != nil {
			var apiErr *fieldservice.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				http.Error(w, apiErr.Error(), apiErr.HTTPStatus)
				return
			}
			http.Error(w, "error capture failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(followUp)
	})

	log.Println("dispatch error service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
