package usecase

import (
	"bufio"
	"fmt"
	"io"
	"nem12-parser/model"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	recordTypeHeader   = "100"
	recordType200      = "200" // NMI/interval block header
	recordType300      = "300" // daily interval readings
	recordType500      = "500" // block trailer (ignored)
	recordType900      = "900" // end of file
	dateLayout         = "20060102"
	minFieldsFor200    = 9  // need at least up to interval length (position 9)
	minFieldsFor300Hdr = 50 // record type + date + 48 intervals
)

// ParseNEM12 streams r line by line and emits one MeterData per NMI block
// (a 200 record and all 300 records under it) via the callback `onBlock`.
// This keeps memory bounded regardless of file size — the caller can insert
// each block immediately (or batch a few) rather than holding the whole file.
func ParseNEM12(r io.Reader, onBlock func(model.MeterData) error) error {
	scanner := bufio.NewScanner(r)

	var current *model.MeterData
	lineNum := 0

	// function for trigger onblock when reach new block nmi200
	// will transform block MeterData into sql insert statement
	// after successful, will flush MeterData
	flush := func() error {
		if current != nil && len(current.MeterConsumptionDetails) > 0 {
			if err := onBlock(*current); err != nil {
				return err
			}
		}
		current = nil
		return nil
	}

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Defensive: handle "500,...,900" glued onto one line (seen in sample data)
		// by splitting off a trailing 900 marker before processing the rest.
		if idx := strings.Index(line, ",900"); idx != -1 && strings.HasSuffix(line, "900") {
			line = line[:idx]
			if line == "" {
				break
			}
		}

		fields := strings.Split(line, ",")
		recordType := fields[0]

		switch recordType {
		case recordTypeHeader:
			// 100 record: file header, nothing to extract for meter_readings.
			continue

		case recordType200:
			// New NMI block begins — flush whatever block we were building.
			if err := flush(); err != nil {
				return err
			}
			md, err := parse200(fields, lineNum)
			if err != nil {
				return err
			}
			current = md

		case recordType300:
			if current == nil {
				return fmt.Errorf("line %d: 300 record found before any 200 record", lineNum)
			}
			details, err := parse300(fields, current.IntervalLengthMinutes, lineNum)
			if err != nil {
				return err
			}
			current.MeterConsumptionDetails = append(current.MeterConsumptionDetails, details...)

		case recordType500:
			// Block trailer — no data needed for meter_readings, ignore.
			continue

		case recordType900:
			// End of file — flush final block and stop.
			if err := flush(); err != nil {
				return err
			}
			return nil

		default:
			// Unknown record type — skip defensively rather than fail the whole file,
			// but this could be tightened to an error depending on how strict we want to be.
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning file: %w", err)
	}

	// File may not end with an explicit 900 marker — flush whatever's left.
	return flush()
}

func parse200(fields []string, lineNum int) (*model.MeterData, error) {
	if len(fields) < minFieldsFor200 {
		return nil, fmt.Errorf("line %d: 200 record has %d fields, expected at least %d", lineNum, len(fields), minFieldsFor200)
	}

	nmi := strings.TrimSpace(fields[1])
	if nmi == "" {
		return nil, fmt.Errorf("line %d: 200 record missing NMI", lineNum)
	}
	if len(nmi) > 10 {
		return nil, fmt.Errorf("line %d: NMI %q exceeds 10 chars", lineNum, nmi)
	}

	intervalLength, err := strconv.Atoi(strings.TrimSpace(fields[8]))
	if err != nil {
		return nil, fmt.Errorf("line %d: invalid interval length %q: %w", lineNum, fields[8], err)
	}
	if intervalLength <= 0 || 1440%intervalLength != 0 {
		return nil, fmt.Errorf("line %d: interval length %d does not divide evenly into a day", lineNum, intervalLength)
	}

	return &model.MeterData{
		NMI:                   nmi,
		IntervalLengthMinutes: intervalLength,
	}, nil
}

func parse300(fields []string, intervalLengthMinutes int, lineNum int) ([]model.MeterConsumptionDetail, error) {
	if intervalLengthMinutes <= 0 {
		return nil, fmt.Errorf("line %d: 300 record found but interval length is unknown", lineNum)
	}

	intervalsPerDay := 1440 / intervalLengthMinutes

	if len(fields) < 2+intervalsPerDay {
		return nil, fmt.Errorf("line %d: 300 record has %d fields, expected at least %d for %d intervals",
			lineNum, len(fields), 2+intervalsPerDay, intervalsPerDay)
	}

	date, err := time.Parse(dateLayout, strings.TrimSpace(fields[1]))
	if err != nil {
		return nil, fmt.Errorf("line %d: invalid date %q: %w", lineNum, fields[1], err)
	}

	details := make([]model.MeterConsumptionDetail, 0, intervalsPerDay)
	for i := 0; i < intervalsPerDay; i++ {
		raw := strings.TrimSpace(fields[2+i])
		consumption, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid consumption value %q at interval %d: %w", lineNum, raw, i+1, err)
		}

		// End-of-interval convention: interval 1 = 00:00-00:30 -> stamped 00:30.
		ts := date.Add(time.Duration(intervalLengthMinutes*(i+1)) * time.Minute)

		details = append(details, model.MeterConsumptionDetail{
			ID:          uuid.New(),
			Timestamp:   ts,
			Consumption: consumption,
		})
	}

	// Everything past position 2+intervalsPerDay (quality flag, reason code,
	// updateDateTime, msatsLoadDateTime) is intentionally ignored — it's
	// administrative metadata not needed for meter_readings, and some fields
	// (like msatsLoadDateTime) are legitimately optional/blank in real files.

	return details, nil
}
