// Package healthdata contains local, versioned Chilean public-health datasets.
package healthdata

import (
	"time"

	"babyapp/backend/internal/models"
)

const (
	PNIScheduleVersion = "PNI Chile 2026 infantil base"
	PNIScheduleSource  = "Ministerio de Salud de Chile - Calendario del Programa Nacional de Inmunizaciones 2026"
	PNIScheduleURL     = "https://vacunas.minsal.cl/wp-content/uploads/2026/01/CALENDARIO-INMUNIZACIONES-2026.pdf"
)

type pniEntry struct {
	Code        string
	Name        string
	AgeLabel    string
	MonthOffset int
	DayOffset   int
}

var infantPNISchedule = []pniEntry{
	{Code: "BCG-RN", Name: "BCG", AgeLabel: "Recién nacido", DayOffset: 0},
	{Code: "HEPB-RN", Name: "Hepatitis B", AgeLabel: "Recién nacido", DayOffset: 0},
	{Code: "HEX-2M", Name: "Hexavalente", AgeLabel: "2 meses", MonthOffset: 2},
	{Code: "NEUMO-2M", Name: "Neumocócica conjugada", AgeLabel: "2 meses", MonthOffset: 2},
	{Code: "HEX-4M", Name: "Hexavalente", AgeLabel: "4 meses", MonthOffset: 4},
	{Code: "NEUMO-4M", Name: "Neumocócica conjugada", AgeLabel: "4 meses", MonthOffset: 4},
	{Code: "HEX-6M", Name: "Hexavalente", AgeLabel: "6 meses", MonthOffset: 6},
	{Code: "MEN-12M", Name: "Meningocócica conjugada", AgeLabel: "12 meses", MonthOffset: 12},
	{Code: "NEUMO-12M", Name: "Neumocócica conjugada", AgeLabel: "12 meses", MonthOffset: 12},
	{Code: "SRP-12M", Name: "Triple vírica", AgeLabel: "12 meses", MonthOffset: 12},
	{Code: "HEX-18M", Name: "Hexavalente", AgeLabel: "18 meses", MonthOffset: 18},
	{Code: "HEPA-18M", Name: "Hepatitis A", AgeLabel: "18 meses", MonthOffset: 18},
	{Code: "VAR-18M", Name: "Varicela", AgeLabel: "18 meses", MonthOffset: 18},
}

// GenerateInfantPNISchedule returns the local PNI schedule for a child birth date.
func GenerateInfantPNISchedule(childID, birthDate string) ([]models.Vaccine, error) {
	birth, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		return nil, err
	}

	vaccines := make([]models.Vaccine, 0, len(infantPNISchedule))
	for _, entry := range infantPNISchedule {
		date := birth.AddDate(0, entry.MonthOffset, entry.DayOffset)
		vaccines = append(vaccines, models.Vaccine{
			ChildID:         childID,
			Code:            entry.Code,
			Name:            entry.Name,
			AgeLabel:        entry.AgeLabel,
			ScheduledDate:   date.Format("2006-01-02"),
			Status:          models.VaccinePending,
			Source:          PNIScheduleSource,
			ScheduleVersion: PNIScheduleVersion,
			CreatedAt:       time.Now(),
		})
	}
	return vaccines, nil
}
