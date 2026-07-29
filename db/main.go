package db

import (
	"os"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Open connects to the sqlite database at dsn and assigns it to DB.
//
// Connect is the normal entry point; this exists so tests can run against an
// in-memory database without touching the filesystem.
func Open(dsn string, debug bool) error {
	c := &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	if debug {
		c.Logger = logger.Default.LogMode(logger.Info)
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(dsn), c)

	return err
}

func Connect(debug bool) {

	//check if database is present
	if _, err := os.Stat("database/sqlite-database.db"); os.IsNotExist(err) {
		//Database does not exist, so create it.
		if err := os.MkdirAll("database", 0o755); err != nil {
			logrus.Fatalf("could not create database directory: %v", err)
		}
		logrus.Info("No database found, creating database/sqlite-database.db")
		file, err := os.Create("database/sqlite-database.db")
		if err != nil {
			logrus.Fatal(err.Error())
		}
		if err := file.Close(); err != nil {
			logrus.Fatalf("could not create database file: %v", err)
		}
		logrus.Info("database/sqlite-database.db created")
	} else {
		//Database exists, moving on.
		logrus.Info("Existing database sqlite-database.db found")
	}

	// Continuing here would leave DB nil and turn the first query into a nil
	// pointer dereference somewhere unrelated.
	if err := Open("database/sqlite-database.db", debug); err != nil {
		logrus.Fatalf("Failed to open the SQLite database: %v", err)
	}
}
