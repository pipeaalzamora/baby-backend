// Package repository manages MongoDB connections and collection access.
package repository

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes creates all necessary indexes for optimal query performance.
// It is idempotent — safe to call on every startup.
func (d *DB) EnsureIndexes(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	type indexSpec struct {
		collection string
		model      mongo.IndexModel
	}

	specs := []indexSpec{
		// users — email único para evitar duplicados
		{
			ColUsers,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "email", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("email_unique"),
			},
		},
		// children — un hijo por usuario
		{
			ColChildren,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "userId", Value: 1}},
				Options: options.Index().SetUnique(true).SetName("userId_unique"),
			},
		},
		// vaccines — listar por hijo ordenado por fecha programada
		{
			ColVaccines,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "scheduledDate", Value: 1}},
				Options: options.Index().SetName("childId_scheduledDate"),
			},
		},
		// measurements — listar por hijo ordenado por fecha
		{
			ColMeasurements,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "date", Value: 1}},
				Options: options.Index().SetName("childId_date"),
			},
		},
		// checkups — listar por hijo
		{
			ColCheckups,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "date", Value: -1}},
				Options: options.Index().SetName("childId_date_desc"),
			},
		},
		// milestones — listar por hijo
		{
			ColMilestones,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "date", Value: -1}},
				Options: options.Index().SetName("childId_date_desc"),
			},
		},
		// diary — listar por hijo
		{
			ColDiary,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "date", Value: -1}},
				Options: options.Index().SetName("childId_date_desc"),
			},
		},
		// medications — listar por hijo
		{
			ColMedications,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "startDate", Value: -1}},
				Options: options.Index().SetName("childId_startDate_desc"),
			},
		},
		// photos — listar por hijo
		{
			ColPhotos,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "date", Value: -1}},
				Options: options.Index().SetName("childId_date_desc"),
			},
		},
		// recipes — listar por hijo y etapa
		{
			ColRecipes,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "stage", Value: 1}},
				Options: options.Index().SetName("childId_stage"),
			},
		},
		// food_introductions — listar por hijo
		{
			ColFoodIntroductions,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}, {Key: "date", Value: -1}},
				Options: options.Index().SetName("childId_date_desc"),
			},
		},
		// notifications — listar por usuario + deduplicación por relatedId
		{
			ColNotifications,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "date", Value: 1}},
				Options: options.Index().SetName("userId_date"),
			},
		},
		{
			ColNotifications,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "relatedId", Value: 1}, {Key: "type", Value: 1}},
				Options: options.Index().SetName("userId_relatedId_type"),
			},
		},
		// caregivers — búsqueda por token de invitación
		{
			ColCaregivers,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "inviteToken", Value: 1}},
				Options: options.Index().SetSparse(true).SetName("inviteToken_sparse"),
			},
		},
		{
			ColCaregivers,
			mongo.IndexModel{
				Keys:    bson.D{{Key: "childId", Value: 1}},
				Options: options.Index().SetName("childId"),
			},
		},
	}

	for _, spec := range specs {
		col := d.db.Collection(spec.collection)
		name, err := col.Indexes().CreateOne(ctx, spec.model)
		if err != nil {
			// Log pero no abortar — un índice existente con el mismo nombre no es error fatal
			log.Printf("⚠️  índice en %q: %v", spec.collection, err)
			continue
		}
		log.Printf("✅ índice %q en colección %q", name, spec.collection)
	}
}
