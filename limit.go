package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
)

type Message struct {
	Status string `json:"status"`
	Body   string `json:"body"`
}
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	mu      sync.Mutex
	clients = make(map[string]*client)
)

func endpointHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	message := Message{
		Status: "Successful",
		Body:   "Hi! You've reached the API. How may I help you?",
	}
	err := json.NewEncoder(writer).Encode(&message)
	if err != nil {
		return
	}
}

// func rateLimiter(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if !limiter.Allow() {
// 			message := Message{
// 				Status: "Request Failed",
// 				Body:   "The API is at capacity, try again later.",
// 			}

// 			w.WriteHeader(http.StatusTooManyRequests)
// 			json.NewEncoder(w).Encode(&message)
// 			return
// 		} else {
// 			next.ServeHTTP(w, r)
// 		}
// 	})
// }

func checkIp() {
	for {
		// Lock the mutex to protect this section from race conditions.
		mu.Lock()
		for ip, client := range clients {
			if time.Since(client.lastSeen) > 3*time.Minute {
				delete(clients, ip)
			}
		}
		mu.Unlock()
	}
}
func perClientRateLimiter(next http.Handler) http.Handler {
	go checkIp()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the IP address from the request.
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Lock the mutex to protect this section from race conditions.
		mu.Lock()
		if _, found := clients[ip]; !found {
			clients[ip] = &client{limiter: rate.NewLimiter(2, 4)}
		}
		clients[ip].lastSeen = time.Now()
		if !clients[ip].limiter.Allow() {
			mu.Unlock()

			message := Message{
				Status: "Request Failed",
				Body:   "The API is at capacity, try again later.",
			}

			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(&message)
			return
		}
		mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func main() {
	r := mux.NewRouter()
	r.Use(perClientRateLimiter)
	//r.HandleFunc("/ping", endpointHandler).Methods("GET")
	r.HandleFunc("/ping", endpointHandler).Methods("GET")
	//r.HandleFunc("/api/unlimited", handleUnlimitedAPI).Methods("GET")
	log.Fatal(http.ListenAndServe(":8000", r))
}
