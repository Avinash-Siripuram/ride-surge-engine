package store

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var ctx = context.Background()

const DriversGeoKey = "drivers:locations"

func InitRedis(redisURL string) {
	var opt *redis.Options
	var err error

	if strings.HasPrefix(redisURL, "redis://") || strings.HasPrefix(redisURL, "rediss://") {
		opt, err = redis.ParseURL(redisURL)
		if err != nil {
			log.Fatalf("Invalid Redis URL: %v", err)
		}
	} else {
		opt = &redis.Options{
			Addr: redisURL,
		}
	}

	RedisClient = redis.NewClient(opt)

	_, err = RedisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully")
}

// UpdateDriverLocation updates driver coordinates using GEOADD
func UpdateDriverLocation(driverID string, lat, lng float64) error {
	err := RedisClient.GeoAdd(ctx, DriversGeoKey, &redis.GeoLocation{
		Name:      driverID,
		Latitude:  lat,
		Longitude: lng,
	}).Err()

	if err != nil {
		return fmt.Errorf("could not update location in Redis: %w", err)
	}
	return nil
}

// FindNearbyDrivers returns active driver geolocations within a given radius in kilometers
func FindNearbyDrivers(lat, lng float64, radiusKm float64) ([]redis.GeoLocation, error) {
	// Query Redis using GeoSearchLocation with GeoSearchLocationQuery
	results, err := RedisClient.GeoSearchLocation(ctx, DriversGeoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("could not search nearby drivers in Redis: %w", err)
	}

	return results, nil
}
