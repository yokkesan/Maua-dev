// create-admin is an offline management command, never a public registration API.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"backend/internal/adminauth"
	"backend/internal/database"
)

func run() error {
	email := flag.String("email", "", "operator email")
	passwordFile := flag.String("password-file", "", "private regular file (owner permissions only); password never accepted as an argument")
	flag.Parse()
	if *email == "" || *passwordFile == "" {
		return fmt.Errorf("--email and --password-file are required")
	}
	f, err := os.OpenFile(*passwordFile, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("cannot open private password file")
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("password file must be regular and inaccessible to group/others")
	}
	data, err := io.ReadAll(io.LimitReader(f, 75))
	if err != nil || len(data) > 74 {
		return fmt.Errorf("invalid password file")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.NewPostgresPool(ctx)
	if err != nil {
		return fmt.Errorf("database unavailable")
	}
	defer db.Close()
	service := adminauth.Service{Repository: &adminauth.Repository{DB: db}}
	if err := service.Create(ctx, *email, password); err != nil {
		return fmt.Errorf("admin creation failed: use a unique email and a password of 12 to 72 bytes")
	}
	fmt.Println("admin account created")
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
