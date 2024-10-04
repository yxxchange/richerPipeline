package database

import (
	"fmt"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

var pipelineDB *gorm.DB

const DsnTemplate = "%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local"
const pipeDbName = "richer"

var ormLogger logger.Interface

func Init() {
	if viper.GetString("metadata.mode") == "debug" {
		ormLogger = logger.Default.LogMode(logger.Info)
	}
	initPipelineDb()
}

func initPipelineDb() {
	user := viper.GetString("database.mysql.pipeline.user")
	passwd := viper.GetString("database.mysql.pipeline.passwd")
	host := viper.GetString("database.mysql.pipeline.host")
	port := viper.GetString("database.mysql.pipeline.port")
	dsn := fmt.Sprintf(DsnTemplate, user, passwd, host, port, pipeDbName)
	gormCfg := &gorm.Config{}
	if ormLogger != nil {
		gormCfg.Logger = ormLogger
	}
	db, err := gorm.Open(mysql.Open(dsn), gormCfg)
	if err != nil {
		panic(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	pipelineDB = db
	return
}
