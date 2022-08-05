package models

import (
	"log"

	"github.com/jmoiron/sqlx"
)

type ProductMst struct {
	ProductMstId int     `db:"productMstId"`
	Name         string  `db:"name"`
	Weight       float64 `db:"weight"`
	Price        float64 `db:"price"`
	ProdUrl      string  `db:"prodUrl"`
	PostageJapan float64 `db:"postageJapan"`
	DeleteFlag   bool    `db:"deleteFlag"`
}

func GetProductMstById(productMstId int) ProductMst {
	var productMst ProductMst
	cmd := "select * from productMst where productMstId = ?"
	row := DbConnection.QueryRowx(cmd, productMstId)
	if err := row.StructScan(&productMst); err != nil {
		log.Fatalln(err)
	}
	return productMst
}

func GetAllProductMst() []ProductMst {
	cmd := `select * from productMst`
	rows, _ := DbConnection.Queryx(cmd)
	defer rows.Close()
	var products []ProductMst
	for rows.Next() {
		var p ProductMst
		rows.StructScan(&p)
		products = append(products, p)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln("GetAllProductMst err")
	}
	return products
}

func GetProductMstListByIds(ids []int) []ProductMst {
	query, args, err := sqlx.In(`select * from productMst where productMstId in (?)`, ids)
	if err != nil {
		log.Fatalln(err)
	}
	query = DbConnection.Rebind(query)
	rows, err := DbConnection.Queryx(query, args...)
	if err != nil {
		log.Fatalln(err)
	}
	defer rows.Close()

	var pMsts []ProductMst
	for rows.Next() {
		var p ProductMst
		if err := rows.StructScan(&p); err != nil {
			log.Fatalln(err)
		}
		pMsts = append(pMsts, p)
	}
	return pMsts
}

func InsertProductMst(name string, weight float64, price float64, postageJapan float64, prodUrl string) {
	cmd := `insert into productMst (name, weight, price, postageJapan, prodUrl, deleteFlag) values (?, ?, ?, ?, ?, ?)`
	_, err := DbConnection.Exec(cmd, name, weight, price, postageJapan, prodUrl, false)
	if err != nil {
		log.Fatalln(err)
	}
}

func UpdateProductMstById(productMstId int, name string, weight float64, price float64, postageJapan float64, prodUrl string) {
	cmd := `update productMst set name = ?, weight = ?, price = ?, postageJapan = ?, prodUrl = ? where productMstId = ?`
	_, err := DbConnection.Exec(cmd, name, weight, price, postageJapan, prodUrl, productMstId)
	if err != nil {
		log.Fatalln("UpdateProductMstById error")
	}
}

func UpdateProductMstDeleteFlagById(id int, flag bool) {
	cmd := `update productMst set deleteFlag = ? where productMstId = ?`
	_, err := DbConnection.Exec(cmd, flag, id)
	if err != nil {
		log.Fatalln(err)
	}
}
