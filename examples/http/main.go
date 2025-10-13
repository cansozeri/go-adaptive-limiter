// HTTP example demonstrating limiter as middleware
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cansozeri/go-adaptive-limiter"
	"github.com/cansozeri/go-adaptive-limiter/algorithm"
	"github.com/cansozeri/go-adaptive-limiter/executor"
)

func main() {
	// Create adaptive limiter
	lim := limiter.New(
		limiter.WithAlgorithm(algorithm.NewVegas(algorithm.VegasConfig{
			MinimumLimit: 10,
			MaxLimit:     100,
			RttNoLoad:    50 * time.Millisecond,
		})),
		limiter.WithExecutor(executor.NewFIFO(executor.FIFOConfig{
			MaxWaitTime: 2 * time.Second,
		})),
	)

	// HTTP handler with limiter
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		err := lim.Execute(r.Context(), func() error {
			// Simulate work
			time.Sleep(100 * time.Millisecond)
			fmt.Fprintf(w, "Request processed successfully\n")
			return nil
		})

		if err != nil {
			http.Error(w, fmt.Sprintf("Request rejected: %v", err), http.StatusTooManyRequests)
			return
		}
	})

	// Stats endpoint
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := lim.Stats()
		fmt.Fprintf(w, "Limiter Stats:\n")
		fmt.Fprintf(w, "  Current Limit: %d\n", stats.CurrentLimit)
		fmt.Fprintf(w, "  In Flight: %d\n", stats.InFlight)
		fmt.Fprintf(w, "  Executing: %d\n", stats.Executing)
	})

	log.Println("Server starting on :8080")
	log.Println("Try: curl http://localhost:8080/api")
	log.Println("Stats: curl http://localhost:8080/stats")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
