package agent

import (
	"log"
	"os"
)

func setupLogging(logFile string) error {
	if logFile == "" {
		return nil
	}

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}

	log.SetOutput(file)
	log.SetFlags(log.LstdFlags)
	return nil
}
