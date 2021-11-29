package main

import (
	"log"
	"os"
)

// logger is a general log
var logger = InitLogger()

func InitLogger() *log.Logger {
	tmpLogger := log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	return tmpLogger
}