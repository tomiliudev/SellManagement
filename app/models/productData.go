package models

import (
	"log"

	"github.com/jmoiron/sqlx"
)

type ProductData struct {
	ProductDataId     int     `db:"productDataId"`
	ProductMstId      int     `db:"productMstId"`
	Count             int     `db:"count"`
	Weight            float64 `db:"weight"`
	Price             float64 `db:"price"`
	PostageChina      float64 `db:"postageChina"`
	GrossProfitMargin float64 `db:"grossProfitMargin"`
	VoucherDataId     int     `db:"voucherDataId"`
}

func GetAllProductData() []ProductData {
	cmd := `select * from productData`
	rows, _ := DbConnection.Queryx(cmd)
	defer rows.Close()
	var products []ProductData
	for rows.Next() {
		var p ProductData
		if err := rows.StructScan(&p); err != nil {
			log.Fatalln("GetAllProductData err 1")
		}
		products = append(products, p)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln("GetAllProductData err 2")
	}
	return products
}

func GetProductDataListByIds(ids []int) []ProductData {
	query, args, err := sqlx.In(`select * from productData where productDataId in (?)`, ids)
	if err != nil {
		log.Fatalln(err)
	}
	query = DbConnection.Rebind(query)
	rows, err := DbConnection.Queryx(query, args...)
	if err != nil {
		log.Fatalln(err)
	}
	defer rows.Close()

	var pDatas []ProductData
	for rows.Next() {
		var p ProductData
		if err := rows.StructScan(&p); err != nil {
			log.Fatalln(err)
		}
		pDatas = append(pDatas, p)
	}
	return pDatas
}

func InsertProductData(productMstId, voucherDataId, count int, weight, price, postageChina, grossProfitMargin float64) int64 {
	cmd := `insert into productData (productMstId, voucherDataId, count, weight, price, postageChina, grossProfitMargin) values (?, ?, ?, ?, ?, ?, ?)`
	res, err := DbConnection.Exec(cmd, productMstId, voucherDataId, count, weight, price, postageChina, grossProfitMargin)
	if err != nil {
		log.Fatalln(err)
	}
	if lastId, err := res.LastInsertId(); err != nil {
		log.Fatalln("InsertProductData err")
	} else {
		return lastId
	}
	return 0
}

func UpdateProductDataById(productDataId, productMstId, count int, weight, price, postageChina, grossProfitMargin float64) {
	cmd := `update productData 
	set productMstId = ?, count = ?, weight = ?, price = ?, 
	postageChina = ?, grossProfitMargin = ?
	where productDataId = ?`
	_, err := DbConnection.Exec(cmd,
		productMstId, count, weight, price,
		postageChina, grossProfitMargin,
		productDataId)
	if err != nil {
		log.Fatalln("UpdateProductData err")
	}
}
