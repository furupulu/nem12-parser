package usecase

import (
	"bufio"
	"fmt"
	"io"

	"nem12-parser/model"
)

// WriteInserts writes standard multi-row INSERT statements for a MeterData block
// to w, batched by batchSize rows per statement, matching the same batching
func WriteInserts(w io.Writer, md model.MeterData, batchSize int) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	details := md.MeterConsumptionDetails
	for start := 0; start < len(details); start += batchSize {
		end := start + batchSize
		if end > len(details) {
			end = len(details)
		}
		if err := writeChunk(bw, md.NMI, details[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func writeChunk(w io.Writer, nmi string, chunk []model.MeterConsumptionDetail) error {
	if _, err := fmt.Fprint(w, `INSERT INTO meter_readings (id, nmi, "timestamp", consumption) VALUES `); err != nil {
		return err
	}

	for i, d := range chunk {
		if i > 0 {
			if _, err := fmt.Fprint(w, ","); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "\n  ('%s', '%s', '%s', %v)",
			d.ID.String(), nmi, d.Timestamp.Format("2006-01-02 15:04:05"), d.Consumption)
		if err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, "\nON CONFLICT ON CONSTRAINT meter_readings_unique_consumption DO NOTHING;\n\n")
	return err
}
