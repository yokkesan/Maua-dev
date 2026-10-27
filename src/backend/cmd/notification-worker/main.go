package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"backend/internal/database"
	"backend/internal/notification"
)

func run() error {
	limit := flag.Int("limit", 50, "maximum notifications per run (1-1000)")
	flag.Parse()
	if *limit < 1 || *limit > 1000 {
		return fmt.Errorf("invalid limit")
	}
	if err := notification.FromEnv().Validate(); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	db, err := database.NewPostgresPool(ctx)
	if err != nil {
		return fmt.Errorf("database unavailable")
	}
	defer db.Close()
	s := notification.Service{Repository: &notification.Repository{DB: db}, Mailer: notification.SMTPFromEnv(), Config: notification.FromEnv()}
	for i := 0; i < *limit; i++ {
		found, err := s.DispatchOne(ctx)
		if err != nil {
			return fmt.Errorf("notification worker failed; inspect event states")
		}
		if !found {
			break
		}
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
