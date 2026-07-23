BINARY_NAME := nem12parser
BUILD_DIR   := bin

INPUT       ?= energymeter.csv
OUTPUT      ?= output.sql

## all: build then run in one command
all: build run

## build: compile the binary into bin/
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

## run: build then run the binary
run: build
	./$(BUILD_DIR)/$(BINARY_NAME) -input $(INPUT) -output $(OUTPUT)

