package util

import (
	"time"

	d_gp "github.com/superpowerdotcom/fhir/go/proto/google/fhir/proto/r4/core/datatypes_go_proto"
)

// AgeResult represents the result of age calculation containing years, months, and days.
// This struct provides a detailed breakdown of age calculations from a birth date.
type AgeResult struct {
	Years  int // Complete years since birth
	Months int // Additional months after complete years
	Days   int // Additional days after complete months
}

// CalculateAgeFromDate calculates the age from a FHIR Date and returns the result as years, months, and days.
// If the date is nil or invalid, it returns an AgeResult with all values set to 0.
// The calculation is performed relative to the current time.
func CalculateAgeFromDate(birthDate *d_gp.Date) AgeResult {
	return CalculateAgeFromDateAt(birthDate, time.Now().UTC())
}

// CalculateAgeFromDateAt calculates the age from a FHIR Date at a specific reference time.
// If the date is nil or invalid, it returns an AgeResult with all values set to 0.
func CalculateAgeFromDateAt(birthDate *d_gp.Date, referenceTime time.Time) AgeResult {
	if birthDate == nil || birthDate.ValueUs == 0 {
		return AgeResult{Years: 0, Months: 0, Days: 0}
	}

	// Convert FHIR Date to time.Time
	return CalculateAgeFromTimeAt(time.UnixMicro(birthDate.ValueUs).UTC(), referenceTime)
}

// CalculateAgeFromTime calculates the age from a time.Time value relative to the current time.
func CalculateAgeFromTime(birthDate time.Time) AgeResult {
	return CalculateAgeFromTimeAt(birthDate, time.Now().UTC())
}

// CalculateAgeFromTimeAt calculates the age from a time.Time value at a specific reference time.
func CalculateAgeFromTimeAt(birthDate time.Time, referenceTime time.Time) AgeResult {
	years := referenceTime.Year() - birthDate.Year()
	months := int(referenceTime.Month()) - int(birthDate.Month())
	days := referenceTime.Day() - birthDate.Day()

	// Adjust for negative days
	if days < 0 {
		months--
		// Get days in previous month
		prevMonth := referenceTime.AddDate(0, -1, 0)
		daysInPrevMonth := time.Date(prevMonth.Year(), prevMonth.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		days += daysInPrevMonth
	}

	// Adjust for negative months
	if months < 0 {
		years--
		months += 12
	}

	return AgeResult{
		Years:  years,
		Months: months,
		Days:   days,
	}
}
