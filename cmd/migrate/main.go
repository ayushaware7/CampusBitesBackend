// Command migrate applies the SQL files in /migrations.
//
//	go run ./cmd/migrate up        apply all pending migrations
//	go run ./cmd/migrate down      roll back the last migration only
//	go run ./cmd/migrate version   print the current version
package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/ayushaware7/CampusBitesBackend/internal/config"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/database"
	"github.com/ayushaware7/CampusBitesBackend/migrations"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: migrate up|down|version")
	}
	dsn, err := config.DSN()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(dsn, true)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	driver, err := migmysql.WithInstance(db, &migmysql.Config{})
	if err != nil {
		log.Fatal(err)
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", driver)
	if err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "version":
		v, dirty, verr := m.Version()
		if verr != nil && !errors.Is(verr, migrate.ErrNilVersion) {
			log.Fatal(verr)
		}
		fmt.Printf("version %d dirty=%v\n", v, dirty)
		return
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no change")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ok")
}
