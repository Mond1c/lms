package database

import (
	"fmt"
	"log"

	"github.com/Mond1c/gitea-classroom/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect(databaseURL string) error {
	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("Database connected successfully")
	return nil
}

func Migrate() error {
	return DB.AutoMigrate(
		&models.User{},
		&models.Course{},
		&models.Assignment{},
		&models.Student{},
		&models.Submission{},
	)
}
