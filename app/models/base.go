package models

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var DbConnection *sqlx.DB

func init() {
	var err error
	DbConnection, err = sqlx.Open("sqlite3", "./sellManagement.sql")
	if err != nil {
		log.Fatalln(err)
	}
}
