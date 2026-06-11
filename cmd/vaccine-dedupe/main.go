package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"babyapp/backend/internal/config"
	"babyapp/backend/internal/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const uniqueVaccineIndexName = "childId_code_scheduledDate_unique"

type duplicateKey struct {
	ChildID       string `bson:"childId" json:"childId"`
	Code          string `bson:"code" json:"code"`
	ScheduledDate string `bson:"scheduledDate" json:"scheduledDate"`
}

type duplicateGroup struct {
	ID    duplicateKey `bson:"_id"`
	Count int          `bson:"count"`
	Docs  []bson.M     `bson:"docs"`
}

type migrationSummary struct {
	Database              string `json:"database"`
	BackupCollection      string `json:"backupCollection,omitempty"`
	TotalBefore           int64  `json:"totalBefore"`
	MissingKeyBefore      int64  `json:"missingKeyBefore"`
	DuplicateGroupsBefore int    `json:"duplicateGroupsBefore"`
	DuplicateDocsBefore   int    `json:"duplicateDocsBefore"`
	DocumentsBackedUp     int    `json:"documentsBackedUp"`
	DocumentsDeleted      int64  `json:"documentsDeleted"`
	KeepersMerged         int    `json:"keepersMerged"`
	DuplicateGroupsAfter  int    `json:"duplicateGroupsAfter"`
	UniqueIndex           string `json:"uniqueIndex"`
}

func main() {
	log.SetFlags(0)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg := config.Load()
	db, err := repository.Connect(ctx, cfg.MongoURI, cfg.DBName)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = db.Disconnect(context.Background())
	}()

	vaccines := db.Collection(repository.ColVaccines)
	totalBefore, err := vaccines.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	missingKeyBefore, err := vaccines.CountDocuments(ctx, missingVaccineKeyFilter())
	if err != nil {
		log.Fatal(err)
	}

	groups, err := findDuplicateGroups(ctx, vaccines)
	if err != nil {
		log.Fatal(err)
	}

	summary := migrationSummary{
		Database:              cfg.DBName,
		TotalBefore:           totalBefore,
		MissingKeyBefore:      missingKeyBefore,
		DuplicateGroupsBefore: len(groups),
		UniqueIndex:           uniqueVaccineIndexName,
	}

	if len(groups) > 0 {
		backupName := fmt.Sprintf("vaccines_duplicates_backup_%s", time.Now().Format("20060102_150405"))
		summary.BackupCollection = backupName
		backup := db.Collection(backupName)

		backupDocs := make([]any, 0)
		deleteIDs := make([]bson.ObjectID, 0)
		for _, group := range groups {
			summary.DuplicateDocsBefore += group.Count
			sortDuplicateDocs(group.Docs)
			keeper := group.Docs[0]
			keeperID, ok := objectID(keeper)
			if !ok {
				log.Fatalf("documento sin ObjectID válido en grupo %+v", group.ID)
			}

			merged := mergedVaccineFields(group.Docs)
			if len(merged) > 0 {
				if _, err := vaccines.UpdateOne(ctx, bson.M{"_id": keeperID}, bson.M{"$set": merged}); err != nil {
					log.Fatal(err)
				}
				summary.KeepersMerged++
			}

			for _, doc := range group.Docs {
				docID, ok := objectID(doc)
				if !ok {
					log.Fatalf("documento sin ObjectID válido en grupo %+v", group.ID)
				}
				willDelete := docID != keeperID
				if willDelete {
					deleteIDs = append(deleteIDs, docID)
				}
				backupDocs = append(backupDocs, backupDocument(doc, group.ID, keeperID, willDelete))
			}
		}

		if len(backupDocs) > 0 {
			if _, err := backup.InsertMany(ctx, backupDocs); err != nil {
				log.Fatal(err)
			}
			summary.DocumentsBackedUp = len(backupDocs)
		}

		if len(deleteIDs) > 0 {
			res, err := vaccines.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": deleteIDs}})
			if err != nil {
				log.Fatal(err)
			}
			summary.DocumentsDeleted = res.DeletedCount
		}
	}

	if err := createUniqueVaccineIndex(ctx, vaccines); err != nil {
		log.Fatal(err)
	}

	afterGroups, err := findDuplicateGroups(ctx, vaccines)
	if err != nil {
		log.Fatal(err)
	}
	summary.DuplicateGroupsAfter = len(afterGroups)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		log.Fatal(err)
	}
}

func findDuplicateGroups(ctx context.Context, vaccines *mongo.Collection) ([]duplicateGroup, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: validVaccineKeyFilter()}},
		{{Key: "$sort", Value: bson.D{
			{Key: "childId", Value: 1},
			{Key: "code", Value: 1},
			{Key: "scheduledDate", Value: 1},
			{Key: "createdAt", Value: 1},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "childId", Value: "$childId"},
				{Key: "code", Value: "$code"},
				{Key: "scheduledDate", Value: "$scheduledDate"},
			}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "docs", Value: bson.D{{Key: "$push", Value: "$$ROOT"}}},
		}}},
		{{Key: "$match", Value: bson.D{{Key: "count", Value: bson.D{{Key: "$gt", Value: 1}}}}}},
	}

	cursor, err := vaccines.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groups []duplicateGroup
	if err := cursor.All(ctx, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func validVaccineKeyFilter() bson.M {
	return bson.M{
		"childId":       bson.M{"$type": "string", "$gt": ""},
		"code":          bson.M{"$type": "string", "$gt": ""},
		"scheduledDate": bson.M{"$type": "string", "$gt": ""},
	}
}

func missingVaccineKeyFilter() bson.M {
	return bson.M{"$or": []bson.M{
		{"childId": bson.M{"$exists": false}},
		{"childId": ""},
		{"code": bson.M{"$exists": false}},
		{"code": ""},
		{"scheduledDate": bson.M{"$exists": false}},
		{"scheduledDate": ""},
	}}
}

func sortDuplicateDocs(docs []bson.M) {
	sort.SliceStable(docs, func(i, j int) bool {
		leftScore := vaccineScore(docs[i])
		rightScore := vaccineScore(docs[j])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		leftCreated := timeField(docs[i], "createdAt")
		rightCreated := timeField(docs[j], "createdAt")
		if !leftCreated.IsZero() && !rightCreated.IsZero() && !leftCreated.Equal(rightCreated) {
			return leftCreated.Before(rightCreated)
		}
		leftID, _ := objectID(docs[i])
		rightID, _ := objectID(docs[j])
		return leftID.Hex() < rightID.Hex()
	})
}

func vaccineScore(doc bson.M) int {
	score := 0
	switch strings.ToLower(stringField(doc, "status")) {
	case "administered":
		score += 1000
	case "skipped":
		score += 100
	}
	for _, field := range []string{"administeredDate", "location", "batchLot", "reactions", "notes"} {
		if stringField(doc, field) != "" {
			score += 50
		}
	}
	for _, field := range []string{"source", "scheduleVersion", "name", "ageLabel"} {
		if stringField(doc, field) != "" {
			score += 5
		}
	}
	return score
}

func mergedVaccineFields(docs []bson.M) bson.M {
	merged := bson.M{}
	status := "pending"
	for _, doc := range docs {
		switch strings.ToLower(stringField(doc, "status")) {
		case "administered":
			status = "administered"
		case "skipped":
			if status != "administered" {
				status = "skipped"
			}
		}
	}
	merged["status"] = status

	for _, field := range []string{
		"name",
		"ageLabel",
		"source",
		"scheduleVersion",
		"administeredDate",
		"location",
		"batchLot",
		"reactions",
		"notes",
	} {
		for _, doc := range docs {
			if value := stringField(doc, field); value != "" {
				merged[field] = value
				break
			}
		}
	}
	return merged
}

func backupDocument(doc bson.M, group duplicateKey, keeperID bson.ObjectID, willDelete bool) bson.M {
	copyDoc := bson.M{}
	for key, value := range doc {
		copyDoc[key] = value
	}
	copyDoc["_dedupeBackup"] = bson.M{
		"migration":  "vaccine-dedupe-20260611",
		"backedUpAt": time.Now(),
		"group": bson.M{
			"childId":       group.ChildID,
			"code":          group.Code,
			"scheduledDate": group.ScheduledDate,
		},
		"keeperId":   keeperID,
		"willDelete": willDelete,
	}
	return copyDoc
}

func createUniqueVaccineIndex(ctx context.Context, vaccines *mongo.Collection) error {
	_, err := vaccines.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "childId", Value: 1},
			{Key: "code", Value: 1},
			{Key: "scheduledDate", Value: 1},
		},
		Options: options.Index().
			SetName(uniqueVaccineIndexName).
			SetUnique(true).
			SetPartialFilterExpression(validVaccineKeyFilter()),
	})
	return err
}

func objectID(doc bson.M) (bson.ObjectID, bool) {
	id, ok := doc["_id"].(bson.ObjectID)
	return id, ok
}

func stringField(doc bson.M, field string) string {
	value, ok := doc[field]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func timeField(doc bson.M, field string) time.Time {
	value, ok := doc[field]
	if !ok || value == nil {
		return time.Time{}
	}
	switch typed := value.(type) {
	case time.Time:
		return typed
	case bson.DateTime:
		return typed.Time()
	default:
		return time.Time{}
	}
}
