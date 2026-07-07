package config

import (
	"core/models"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var db *gorm.DB
var err error

func DbInit() {
	config := GetConfig()
	connectString := fmt.Sprintf(config.Dburl)

	// Open the connection to the database
	db, err = gorm.Open(postgres.Open(connectString), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})

	if err != nil {
		log.Fatalf("DB Connection Error: %v", err)
	}

	fmt.Println("Connected to Database")

	err = db.AutoMigrate(
		&models.Waitlist{},
		&models.User{},
		&models.Role{},
		&models.Workspace{},
		&models.Channels{},
		&models.ManageChannels{},
		&models.Credentials{},
		&models.Customer{},
		&models.Payment{},
		&models.GitHubInstallation{},
		&models.ManageWorkspace{},
		&models.GitHubRepository{},
		&models.Payment{},
		&models.GitHubCommits{},
		&models.GitHubCommitFiles{},
		&models.CommitFileEmbedding{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println(config.DbHost, config.DbName)
}

func DbManager() *gorm.DB {
	return db
}
