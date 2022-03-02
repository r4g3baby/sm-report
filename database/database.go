package database

import (
	"fmt"
	"github.com/r4g3baby/sm-report/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func init() {
	var err error
	if DB, err = gorm.Open(mysql.Open(config.Config.Database), &gorm.Config{
		Logger: logger.Discard,
	}); err != nil {
		panic(fmt.Errorf("error connecting to database: %w", err))
	}

	if err := DB.AutoMigrate(&Report{}, &Comment{}); err != nil {
		panic(fmt.Errorf("error migrating database: %w", err))
	}
}
