// Package models defines all domain types used across the application.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ─── Child ────────────────────────────────────────────────────────────────────

// Child represents the baby's profile.
type Child struct {
	ID              bson.ObjectID `bson:"_id,omitempty"    json:"id"`
	UserID          string        `bson:"userId"           json:"userId"`
	Name            string        `bson:"name"             json:"name"`
	BirthDate       string        `bson:"birthDate"        json:"birthDate"`
	Gender          string        `bson:"gender"           json:"gender"` // "M" | "F"
	BloodType       string        `bson:"bloodType,omitempty" json:"bloodType,omitempty"`
	PhotoURL        string        `bson:"photoUrl,omitempty"  json:"photoUrl,omitempty"`
	BirthWeightKg   float64       `bson:"birthWeightKg"    json:"birthWeightKg"`
	BirthHeightCm   float64       `bson:"birthHeightCm"    json:"birthHeightCm"`
	CreatedAt       time.Time     `bson:"createdAt"        json:"createdAt"`
	UpdatedAt       time.Time     `bson:"updatedAt"        json:"updatedAt"`
}

// ─── User ─────────────────────────────────────────────────────────────────────

// User represents an authenticated parent/caregiver account.
type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string        `bson:"email"         json:"email"`
	PasswordHash string        `bson:"passwordHash"  json:"-"`
	Name         string        `bson:"name"          json:"name"`
	ChildID      string        `bson:"childId,omitempty" json:"childId,omitempty"`
	CreatedAt    time.Time     `bson:"createdAt"     json:"createdAt"`
}

// ─── Vaccine ──────────────────────────────────────────────────────────────────

// VaccineStatus represents the administration state of a vaccine.
type VaccineStatus string

const (
	VaccinePending      VaccineStatus = "pending"
	VaccineAdministered VaccineStatus = "administered"
	VaccineSkipped      VaccineStatus = "skipped"
)

// Vaccine represents a single vaccine in the PNI schedule.
type Vaccine struct {
	ID               bson.ObjectID `bson:"_id,omitempty"          json:"id"`
	ChildID          string        `bson:"childId"                json:"childId"`
	Code             string        `bson:"code"                   json:"code"`
	Name             string        `bson:"name"                   json:"name"`
	AgeLabel         string        `bson:"ageLabel"               json:"ageLabel"`
	ScheduledDate    string        `bson:"scheduledDate"          json:"scheduledDate"`
	AdministeredDate string        `bson:"administeredDate,omitempty" json:"administeredDate,omitempty"`
	Status           VaccineStatus `bson:"status"                 json:"status"`
	Location         string        `bson:"location,omitempty"     json:"location,omitempty"`
	BatchLot         string        `bson:"batchLot,omitempty"     json:"batchLot,omitempty"`
	Reactions        string        `bson:"reactions,omitempty"    json:"reactions,omitempty"`
	Notes            string        `bson:"notes,omitempty"        json:"notes,omitempty"`
	CreatedAt        time.Time     `bson:"createdAt"              json:"createdAt"`
}

// ─── Measurement ──────────────────────────────────────────────────────────────

// Measurement records a growth measurement at a point in time.
type Measurement struct {
	ID                   bson.ObjectID `bson:"_id,omitempty"              json:"id"`
	ChildID              string        `bson:"childId"                    json:"childId"`
	Date                 string        `bson:"date"                       json:"date"`
	WeightKg             float64       `bson:"weightKg"                   json:"weightKg"`
	HeightCm             float64       `bson:"heightCm"                   json:"heightCm"`
	HeadCircumferenceCm  float64       `bson:"headCircumferenceCm"        json:"headCircumferenceCm"`
	PercentileWeight     *float64      `bson:"percentileWeight,omitempty" json:"percentileWeight,omitempty"`
	PercentileHeight     *float64      `bson:"percentileHeight,omitempty" json:"percentileHeight,omitempty"`
	CreatedAt            time.Time     `bson:"createdAt"                  json:"createdAt"`
}

// ─── Checkup ──────────────────────────────────────────────────────────────────

// Prescription is a medication prescribed during a checkup.
type Prescription struct {
	Medication string `bson:"medication" json:"medication"`
	Dosage     string `bson:"dosage"     json:"dosage"`
	Duration   string `bson:"duration"   json:"duration"`
}

// Checkup represents a pediatric visit.
type Checkup struct {
	ID              bson.ObjectID  `bson:"_id,omitempty"              json:"id"`
	ChildID         string         `bson:"childId"                    json:"childId"`
	Date            string         `bson:"date"                       json:"date"`
	DoctorName      string         `bson:"doctorName"                 json:"doctorName"`
	Center          string         `bson:"center"                     json:"center"`
	Observations    string         `bson:"observations"               json:"observations"`
	Prescriptions   []Prescription `bson:"prescriptions"              json:"prescriptions"`
	NextAppointment string         `bson:"nextAppointment,omitempty"  json:"nextAppointment,omitempty"`
	CreatedAt       time.Time      `bson:"createdAt"                  json:"createdAt"`
}

// ─── Milestone ────────────────────────────────────────────────────────────────

// MilestoneCategory classifies developmental milestones.
type MilestoneCategory string

const (
	MilestoneMotor    MilestoneCategory = "motor"
	MilestoneSocial   MilestoneCategory = "social"
	MilestoneLanguage MilestoneCategory = "language"
	MilestoneCognitive MilestoneCategory = "cognitive"
	MilestoneFeeding  MilestoneCategory = "feeding"
)

// Milestone records a developmental achievement.
type Milestone struct {
	ID          bson.ObjectID     `bson:"_id,omitempty" json:"id"`
	ChildID     string            `bson:"childId"       json:"childId"`
	Date        string            `bson:"date"          json:"date"`
	Category    MilestoneCategory `bson:"category"      json:"category"`
	Title       string            `bson:"title"         json:"title"`
	Description string            `bson:"description"   json:"description"`
	MediaURLs   []string          `bson:"mediaUrls"     json:"mediaUrls"`
	CreatedAt   time.Time         `bson:"createdAt"     json:"createdAt"`
}

// ─── DiaryEntry ───────────────────────────────────────────────────────────────

// DiaryType classifies diary entries.
type DiaryType string

const (
	DiaryFeeding DiaryType = "feeding"
	DiarySleep   DiaryType = "sleep"
	DiaryDiaper  DiaryType = "diaper"
	DiaryMood    DiaryType = "mood"
	DiaryNote    DiaryType = "note"
)

// DiaryEntry records a daily event (feeding, sleep, diaper, mood, note).
type DiaryEntry struct {
	ID        bson.ObjectID          `bson:"_id,omitempty"      json:"id"`
	ChildID   string                 `bson:"childId"            json:"childId"`
	Date      string                 `bson:"date"               json:"date"`
	Type      DiaryType              `bson:"type"               json:"type"`
	Data      map[string]interface{} `bson:"data"               json:"data"`
	Notes     string                 `bson:"notes,omitempty"    json:"notes,omitempty"`
	CreatedAt time.Time              `bson:"createdAt"          json:"createdAt"`
}

// ─── Medication ───────────────────────────────────────────────────────────────

// Medication represents a prescribed treatment.
type Medication struct {
	ID             bson.ObjectID `bson:"_id,omitempty"          json:"id"`
	ChildID        string        `bson:"childId"                json:"childId"`
	Name           string        `bson:"name"                   json:"name"`
	Dosage         string        `bson:"dosage"                 json:"dosage"`
	FrequencyHours int           `bson:"frequencyHours"         json:"frequencyHours"`
	StartDate      string        `bson:"startDate"              json:"startDate"`
	EndDate        string        `bson:"endDate,omitempty"      json:"endDate,omitempty"`
	Active         bool          `bson:"active"                 json:"active"`
	PrescribedBy   string        `bson:"prescribedBy"           json:"prescribedBy"`
	Reason         string        `bson:"reason"                 json:"reason"`
	CreatedAt      time.Time     `bson:"createdAt"              json:"createdAt"`
}

// ─── Photo ────────────────────────────────────────────────────────────────────

// Photo stores metadata for an uploaded image.
type Photo struct {
	ID        bson.ObjectID `bson:"_id,omitempty"       json:"id"`
	ChildID   string        `bson:"childId"             json:"childId"`
	URL       string        `bson:"url"                 json:"url"`
	Date      string        `bson:"date"                json:"date"`
	Tags      []string      `bson:"tags"                json:"tags"`
	Caption   string        `bson:"caption,omitempty"   json:"caption,omitempty"`
	CreatedAt time.Time     `bson:"createdAt"           json:"createdAt"`
}

// ─── Recipe ───────────────────────────────────────────────────────────────────

// FoodStage represents the minimum age stage for a recipe.
type FoodStage string

// RecipeIngredient is a single ingredient with amount.
type RecipeIngredient struct {
	Name   string `bson:"name"   json:"name"`
	Amount string `bson:"amount" json:"amount"`
}

// Recipe is a baby food recipe for a specific developmental stage.
type Recipe struct {
	ID                  bson.ObjectID      `bson:"_id,omitempty"              json:"id"`
	ChildID             string             `bson:"childId"                    json:"childId"`
	Name                string             `bson:"name"                       json:"name"`
	Stage               FoodStage          `bson:"stage"                      json:"stage"`
	Texture             string             `bson:"texture"                    json:"texture"`
	Ingredients         []RecipeIngredient `bson:"ingredients"                json:"ingredients"`
	Steps               []string           `bson:"steps"                      json:"steps"`
	NutritionHighlights []string           `bson:"nutritionHighlights"        json:"nutritionHighlights"`
	Allergens           []string           `bson:"allergens"                  json:"allergens"`
	PrepTimeMin         int                `bson:"prepTimeMin"                json:"prepTimeMin"`
	ImageURL            string             `bson:"imageUrl,omitempty"         json:"imageUrl,omitempty"`
	IsFavorite          bool               `bson:"isFavorite"                 json:"isFavorite"`
	CreatedAt           time.Time          `bson:"createdAt"                  json:"createdAt"`
}

// FoodIntroduction records when a new food was introduced and the reaction.
type FoodIntroduction struct {
	ID        bson.ObjectID `bson:"_id,omitempty"       json:"id"`
	ChildID   string        `bson:"childId"             json:"childId"`
	FoodName  string        `bson:"foodName"            json:"foodName"`
	Date      string        `bson:"date"                json:"date"`
	Reaction  string        `bson:"reaction"            json:"reaction"` // none|mild|moderate|severe
	Notes     string        `bson:"notes,omitempty"     json:"notes,omitempty"`
	Accepted  bool          `bson:"accepted"            json:"accepted"`
	CreatedAt time.Time     `bson:"createdAt"           json:"createdAt"`
}

// ─── Notification ─────────────────────────────────────────────────────────────

// NotificationType classifies app notifications.
type NotificationType string

const (
	NotifVaccine           NotificationType = "vaccine"
	NotifCheckup           NotificationType = "checkup"
	NotifMedication        NotificationType = "medication"
	NotifMilestoneReminder NotificationType = "milestone_reminder"
)

// AppNotification is an in-app alert for the user.
type AppNotification struct {
	ID        bson.ObjectID    `bson:"_id,omitempty"       json:"id"`
	UserID    string           `bson:"userId"              json:"userId"`
	ChildID   string           `bson:"childId,omitempty"   json:"childId,omitempty"`
	Type      NotificationType `bson:"type"                json:"type"`
	Title     string           `bson:"title"               json:"title"`
	Message   string           `bson:"message"             json:"message"`
	Date      string           `bson:"date"                json:"date"`
	Read      bool             `bson:"read"                json:"read"`
	RelatedID string           `bson:"relatedId,omitempty" json:"relatedId,omitempty"`
	CreatedAt time.Time        `bson:"createdAt"           json:"createdAt"`
}

// ─── Caregiver ────────────────────────────────────────────────────────────────

// CaregiverRole defines the access level of a caregiver.
type CaregiverRole string

const (
	RoleParent    CaregiverRole = "parent"
	RoleCaregiver CaregiverRole = "caregiver"
	RoleDoctor    CaregiverRole = "doctor"
	RoleViewer    CaregiverRole = "viewer"
)

// Caregiver represents a person with shared access to the child's data.
type Caregiver struct {
	ID          bson.ObjectID `bson:"_id,omitempty"           json:"id"`
	ChildID     string        `bson:"childId"                 json:"childId"`
	Email       string        `bson:"email"                   json:"email"`
	Name        string        `bson:"name"                    json:"name"`
	Role        CaregiverRole `bson:"role"                    json:"role"`
	InvitedAt   string        `bson:"invitedAt"               json:"invitedAt"`
	AcceptedAt  string        `bson:"acceptedAt,omitempty"    json:"acceptedAt,omitempty"`
	AvatarURL   string        `bson:"avatarUrl,omitempty"     json:"avatarUrl,omitempty"`
	InviteToken string        `bson:"inviteToken,omitempty"   json:"-"` // never expose in JSON
	CreatedAt   time.Time     `bson:"createdAt"               json:"createdAt"`
}
