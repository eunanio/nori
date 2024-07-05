package e

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/eunanhardy/nori/internal/paths"
)

func init(){
	homeDir, err  := os.UserHomeDir(); if err != nil {
		fmt.Println("Error getting user home directory: ", err)
	}
	configPath := fmt.Sprintf("%s/.nori", homeDir)
	paths.MkDirIfNotExist(configPath)
	logPath := fmt.Sprintf("%s/nori.log",configPath)
	logFile, err := os.OpenFile(logPath,os.O_CREATE | os.O_WRONLY | os.O_APPEND, 0666); if err != nil {
		panic(err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger := slog.New(slog.NewJSONHandler(multiWriter,nil))
	slog.SetDefault(logger)
}

func Resolve(err error, msg string) error {
	_, ok := os.LookupEnv("NORI_DEBUG")
	if err != nil {
		if ok {
			debug.PrintStack()
		}

		if msg != "" {
			slog.Error(msg, "error", err.Error())
			os.Exit(1)
		}
		slog.Error(err.Error())
		os.Exit(1)
	}
	return err
}

func Fatal(err error, msg string) {
	_, ok := os.LookupEnv("NORI_DEBUG")
	if err != nil {
		if ok {
			debug.PrintStack()
		}

		if msg != "" {
			slog.Error(msg, "error",err.Error())
			os.Exit(1)
		}
		slog.Error(msg, "error",err.Error())
		os.Exit(1)
	}
}