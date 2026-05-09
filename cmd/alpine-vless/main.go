package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkssssss/alpine-vless/internal/app"
)

func main() {
	// Ignore SIGHUP so update tasks are less likely to be interrupted by SSH disconnect.
	signal.Ignore(syscall.SIGHUP)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(os.Args) == 2 && os.Args[1] == app.UpdateWorkerArg {
		if err := app.RunUpdateSingBoxWorker(ctx, os.Stdout, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err.Error())
			os.Exit(1)
		}
		return
	}

	if err := app.Run(ctx, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
