package models

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DbConnection *sql.DB

func init() {
	var err error
	DbConnection, err = sql.Open("sqlite3", "./sellManagement.sql")
	if err != nil {
		log.Fatalln(err)
	}
}
