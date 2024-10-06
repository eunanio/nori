package console

import (
	"fmt"
	"os"
)

const (
	seperator        = "◆|"
	seperatorError   = "×|"
	seperatorSuccess = "✓|"
)

func Println(msg string) {
	fmt.Fprintf(os.Stdout, "%s  %s\n", seperator, msg)
}

func Print(msg string) {
	fmt.Fprintf(os.Stdout, "%s  %s", seperator, msg)
}

func Success(msg string) {
	fmt.Fprintf(os.Stdout, "\u001b[32m%s  %s\u001b[0m\n", seperatorSuccess, msg)
}

func Error(msg string) {
	fmt.Fprintf(os.Stderr, "\u001b[31m%s  %s\u001b[0m\n", seperatorError, msg)
}

func Debug(msg string) {
	if _, ok := os.LookupEnv("NORI_DEBUG"); ok {
		fmt.Fprintf(os.Stdout, "\u001b[33m%s\u001b[0m\n", msg)
	}
}
