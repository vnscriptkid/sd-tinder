package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
)

var (
	rdb  *redis.Client
	ctx  = context.Background()
	port = "8080"
)

type LocationUpdate struct {
	UserID    string  `json:"userId"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type NearbyUsersRequest struct {
	UserID string  `json:"userId"`
	Radius float64 `json:"radius"`
}

func main() {
	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	pong := rdb.Ping(ctx)
	if pong.Err() != nil {
		log.Fatalf("Failed to connect to Redis: %v", pong.Err())
	}

	r := mux.NewRouter()
	r.HandleFunc("/locations", updateLocation).Methods("POST")
	r.HandleFunc("/locations", getNearbyUsers).Methods("GET")

	log.Printf("Server running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func updateLocation(w http.ResponseWriter, r *http.Request) {
	var locUpdate LocationUpdate
	if err := json.NewDecoder(r.Body).Decode(&locUpdate); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if locUpdate.UserID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	if locUpdate.Latitude == 0 || locUpdate.Longitude == 0 {
		http.Error(w, "latitude and longitude are required", http.StatusBadRequest)
		return
	}

	// Fill timestamp on the server side
	timestamp := time.Now().Format(time.RFC3339)

	// GEOADD userLocations <longitude> <latitude> <userId>
	_, err := rdb.GeoAdd(ctx, "userLocations", &redis.GeoLocation{
		Name:      locUpdate.UserID,
		Longitude: locUpdate.Longitude,
		Latitude:  locUpdate.Latitude,
	}).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Optionally store the timestamp if needed
	rdb.HSet(ctx, fmt.Sprintf("user:%s", locUpdate.UserID), "lastUpdate", timestamp)

	response := map[string]string{"status": "success", "message": "Location updated successfully"}
	json.NewEncoder(w).Encode(response)
}

func getNearbyUsers(w http.ResponseWriter, r *http.Request) {
	userId := r.URL.Query().Get("userId")
	radiusStr := r.URL.Query().Get("radius")
	radius, err := strconv.ParseFloat(radiusStr, 64)
	if err != nil {
		http.Error(w, "Invalid radius", http.StatusBadRequest)
		return
	}

	// Get user location
	userLocation, err := rdb.GeoPos(ctx, "userLocations", userId).Result()
	if err != nil || len(userLocation) == 0 || userLocation[0] == nil {
		http.Error(w, "User location not found", http.StatusNotFound)
		return
	}

	longitude := userLocation[0].Longitude
	latitude := userLocation[0].Latitude

	// Use geo search instead of radius query
	locations, err := rdb.GeoSearchLocation(ctx, "userLocations", &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude: longitude,
			Latitude:  latitude,
			Radius:    radius,
			Count:     10,
			// RadiusUnit: "km",
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var nearbyUsers []map[string]interface{}
	for _, loc := range locations {
		nearbyUsers = append(nearbyUsers, map[string]interface{}{
			"userId":    loc.Name,
			"latitude":  loc.Latitude,
			"longitude": loc.Longitude,
			"distance":  loc.Dist,
		})
	}

	response := map[string]interface{}{"status": "success", "nearbyUsers": nearbyUsers}
	json.NewEncoder(w).Encode(response)
}
