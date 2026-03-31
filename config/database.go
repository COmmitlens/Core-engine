package config

import (
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
	// Core user/auth tables
	// &models.User{},
	// &models.Role{},
	// // Workspace/channel tables
	// &models.Workspace{},
	// &models.Channels{},
	// &models.ManageWorkspace{},
	// &models.ManageChannels{},
	// &models.Credentials{},
	// // GitHub integration tables
	// &models.GitHubInstallation{},
	// &models.GitHubRepository{},
	// &models.GitHubCommits{},
	// &models.GitHubCommitFiles{},
	// &models.CommitFileEmbedding{},
	// // Billing/payment tables
	// &models.CreditOption{},
	// &models.Subscription{},
	// &models.UserCredit{},
	// &models.UserCreditHistory{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println(config.DbHost, config.DbName)
}

func DbManager() *gorm.DB {
	return db
}
