package main

import (
	"flag"
	"grindcode/floenergy/model"
	"grindcode/floenergy/usecase"
	"log"
	"os"
)

func main() {
	inputPath := flag.String("input", "energymeter.csv", "path to NEM12 input file")
	outputPath := flag.String("output", "output.sql", "path to write generated INSERT statements")
	flag.Parse()

	file, err := os.Open(*inputPath)
	if err != nil {
		log.Fatalf("open input file: %v", err)
	}
	defer file.Close()

	outFile, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("create output file: %v", err)
	}
	defer outFile.Close()

	totalRows := 0
	err = usecase.ParseNEM12(file, func(md model.MeterData) error {
		if err := usecase.WriteInserts(outFile, md, 1000); err != nil {
			return err
		}

		totalRows += len(md.MeterConsumptionDetails)
		log.Printf("NMI %s: %d readings processed", md.NMI, len(md.MeterConsumptionDetails))
		return nil
	})
	if err != nil {
		log.Fatalf("parse NEM12 file: %v", err)
	}

	log.Printf("done — %d total readings written to %s", totalRows, *outputPath)
}
