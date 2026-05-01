// Package repository manages MongoDB connections and collection access.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DB wraps the MongoDB client and exposes typed collection accessors.
type DB struct {
	client *mongo.Client
	db     *mongo.Database
}

// Connect establishes a MongoDB connection and pings the server.
func Connect(ctx context.Context, uri, dbName string) (*DB, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &DB{client: client, db: client.Database(dbName)}, nil
}

// Disconnect closes the MongoDB connection gracefully.
func (d *DB) Disconnect(ctx context.Context) error {
	return d.client.Disconnect(ctx)
}

// Collection returns a typed collection by name.
func (d *DB) Collection(name string) *mongo.Collection {
	return d.db.Collection(name)
}

// Collection name constants — single source of truth.
const (
	ColUsers             = "users"
	ColChildren          = "children"
	ColVaccines          = "vaccines"
	ColMeasurements      = "measurements"
	ColCheckups          = "checkups"
	ColMilestones        = "milestones"
	ColDiary             = "diary"
	ColMedications       = "medications"
	ColPhotos            = "photos"
	ColRecipes           = "recipes"
	ColFoodIntroductions = "food_introductions"
	ColNotifications     = "notifications"
	ColCaregivers        = "caregivers"
)
