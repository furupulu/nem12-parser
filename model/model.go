package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	DBConn string = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	DBName string = "flo_energy"
)

type MeterData struct {
	NMI                     string `db:"nmi"`
	IntervalLengthMinutes   int
	MeterConsumptionDetails []MeterConsumptionDetail
}

type MeterConsumptionDetail struct {
	ID          uuid.UUID `db:"id"`
	Timestamp   time.Time `db:"timestamp"`
	Consumption float64   `db:"consumption"`
}
