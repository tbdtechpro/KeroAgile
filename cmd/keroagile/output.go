package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
		os.Exit(1)
	}
}

func printError(code int, msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+msg+"\n", args...)
	os.Exit(code)
}

func exitNotFound(id string) {
	printError(1, "%s not found", id)
}

func exitValidation(msg string) {
	printError(2, msg)
}
