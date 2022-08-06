package models

import (
	"log"

	"github.com/jmoiron/sqlx"
)

type ProductDetailData struct {
	ProductDetailDataId int     `db:"productDetailDataId"`
	ProductDataId       int     `db:"productDataId"`
	GrossProfitMargin   float64 `db:"grossProfitMargin"`
	PurchaseDate        string  `db:"purchaseDate"`
	SalesDate           string  `db:"salesDate"`
}

func GetAllProductDetailData() []ProductDetailData {
	cmd := `select * from productDetailData`
	rows, _ := DbConnection.Queryx(cmd)
	defer rows.Close()
	var details []ProductDetailData
	for rows.Next() {
		var p ProductDetailData
		if err := rows.StructScan(&p); err != nil {
			log.Fatalln("GetAllProductDetailData err 1")
		}
		details = append(details, p)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln("GetAllProductDetailData err 2")
	}
	return details
}

func GetProductDetailDataByProductDataId(productDataId int) []ProductDetailData {
	cmd := `select * from productDetailData where productDataId = ?`
	rows, _ := DbConnection.Queryx(cmd, productDataId)
	defer rows.Close()
	var details []ProductDetailData
	for rows.Next() {
		var p ProductDetailData
		if err := rows.StructScan(&p); err != nil {
			log.Fatalln("GetProductDetailDataByProductDataId err")
		}
		details = append(details, p)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln(err)
	}
	return details
}

func InsertProductDetailData(productDataId int64, grossProfitMargin float64) {
	cmd := `insert into productDetailData (productDataId, grossProfitMargin) values (?, ?)`
	_, err := DbConnection.Exec(cmd, productDataId, grossProfitMargin)
	if err != nil {
		log.Fatalln(err)
	}
}

func UpdateGrossProfitMarginById(productDetailDataId int, grossProfitMargin float64) {
	cmd := `update productDetailData set grossProfitMargin = ? where productDetailDataId = ?`
	_, err := DbConnection.Exec(cmd, grossProfitMargin, productDetailDataId)
	if err != nil {
		log.Fatalln(err)
	}
}

func UpdateGrossProfitMarginByProductDataId(productDataId int, grossProfitMargin float64) {
	cmd := `update productDetailData set grossProfitMargin = ? where productDataId = ?`
	_, err := DbConnection.Exec(cmd, grossProfitMargin, productDataId)
	if err != nil {
		log.Fatalln(err)
	}
}

func UpdateSalesDateInIds(salesDate string, productDetailDataIds []int) {
	cmd := `update productDetailData set salesDate = ? where productDetailDataId in (?)`
	query, args, err := sqlx.In(cmd, salesDate, productDetailDataIds)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = DbConnection.Exec(query, args...)
	if err != nil {
		log.Fatalln(err)
	}
}
