package main

import (
	"bufio"
	"log/slog"
	"os"

	"github.com/WangYihang/uio"
)

func main() {
	if len(os.Args) != 2 {
		slog.Error("Usage: uio <uri>")
		os.Exit(1)
	}

	fd, err := uio.Open(os.Args[1])
	if err != nil {
		slog.Error("Failed to open resource", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		slog.Info("Read line", slog.String("line", scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Failed to read resource", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
