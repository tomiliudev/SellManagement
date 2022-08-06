package models

import (
	"log"

	"github.com/ahmetalpbalkan/go-linq"
)

type VoucherData struct {
	VoucherDataId  int     `db:"voucherDataId"`
	OrderId        string  `db:"orderId"`
	OrderTime      string  `db:"orderTime"`
	ShippingMode   int     `db:"shippingMode"`
	ShippingMethod int     `db:"shippingMethod"`
	DeliveryId     string  `db:"deliveryId"`
	Weight         float64 `db:"weight"`
	Cost           float64 `db:"cost"`
	Status         int     `db:"status"`
	DeleteFlag     bool    `db:"deleteFlag"`
}

func GetAllVoucherData() []VoucherData {
	cmd := `select * from voucherData`
	rows, _ := DbConnection.Queryx(cmd)
	defer rows.Close()
	var vouchers []VoucherData
	for rows.Next() {
		var v VoucherData
		rows.StructScan(&v)
		vouchers = append(vouchers, v)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln("GetAllVoucherData err")
	}
	return vouchers
}

func GetVoucherDataById(voucherDataId int) VoucherData {
	voucherDataList := GetAllVoucherData()
	voucherData := linq.From(voucherDataList).FirstWith(func(i interface{}) bool {
		return i.(VoucherData).VoucherDataId == voucherDataId
	})
	return voucherData.(VoucherData)
}

func InsertVoucherData(orderId string, orderTime string, shippingMode, shippingMethod int, deliveryId string, weight, cost float64, status int) {
	cmd := `insert into voucherData (orderId,orderTime,shippingMode,shippingMethod,deliveryId,weight,cost,status,deleteFlag) values (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := DbConnection.Exec(cmd, orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status, false)
	if err != nil {
		log.Fatalln(err)
	}
}

func UpdateVoucherData(voucherDataId int, orderId string, orderTime string, shippingMode, shippingMethod int, deliveryId string, weight, cost float64, status int) {
	cmd := `update voucherData 
		set orderId = ?, orderTime = ?, shippingMode = ?, shippingMethod = ?, 
		deliveryId = ?, weight = ?, cost = ?, status = ?
		where voucherDataId = ?`
	_, err := DbConnection.Exec(cmd, orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status, voucherDataId)
	if err != nil {
		log.Fatalln(err)
	}
}

func DeleteVoucherDataById(voucherDataId int) {
	cmd := `update voucherData set deleteFlag = true where voucherDataId = ?`
	DbConnection.Exec(cmd, voucherDataId)
}
