package database

import (
	"context"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var DB *gorm.DB

func Init() {
	if DB == nil {
		var err error

		dbCredential := os.Getenv("DB_CREDENTIAL")
		if dbCredential == "" {
			log.Fatal("DB_CREDENTIAL environment variable is not set")
		}

		DB, err = gorm.Open(sqlite.Open(dbCredential), &gorm.Config{
			SkipDefaultTransaction:                   true,
			PrepareStmt:                              true,
			DisableForeignKeyConstraintWhenMigrating: true,
			NamingStrategy: schema.NamingStrategy{
				TablePrefix:   "",
				SingularTable: true,
				NoLowerCase:   true,
			},
		})

		if err != nil {
			log.Fatal("Failed open connection")
		}

		Pool, er := DB.DB()
		if er != nil {
			log.Fatal("Failed create connection pool")
		}
		Pool.SetMaxOpenConns(10)
		Pool.SetMaxIdleConns(5)

		DB.Exec("PRAGMA journal_mode = WAL;")
		DB.Exec("PRAGMA synchronous = NORMAL;")
	}
}

func SetContext(ctx context.Context) {
	DB = DB.WithContext(ctx)
}
