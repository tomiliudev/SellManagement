package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"database/sql"

	"github.com/ahmetalpbalkan/go-linq"
	"github.com/leekchan/timeutil"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/ini.v1"
)

type ConfigList struct {
	ShippingMode   []string `ini:"shippingMode"`
	ShippingMethod []string `ini:"shippingMethod"`
	Status         []string `ini:"status"`
}

var Config ConfigList

type VoucherData struct {
	VoucherDataId  int
	OrderId        string
	OrderTime      string
	ShippingMode   int
	ShippingMethod int
	DeliveryId     string
	Weight         float64
	Cost           float64
	Status         int
	DeleteFlag     bool
}

type VoucherData2 struct {
	VoucherDataId      int
	OrderId            string
	OrderTime          string
	ShippingMode       int
	ShippingModeName   string
	ShippingMethod     int
	ShippingMethodName string
	DeliveryId         string
	Weight             float64
	Cost               float64
	Status             int
	StatusName         string
	DeleteFlag         bool
}

type ProductMst struct {
	ProductMstId int
	Name         string
	Weight       float64
	Price        float64
	ProdUrl      string
	PostageJapan float64
	DeleteFlag   bool
}

type ProductData struct {
	ProductDataId     int
	ProductMstId      int
	Count             int
	Weight            float64
	Price             float64
	PostageChina      float64
	GrossProfitMargin float64
	VoucherDataId     int
}

// 表示用
type ProductData2 struct {
	ProductDataId     int
	ProductMstId      int
	Name              string
	Count             int
	Weight            float64
	Price             float64
	PostageChina      float64
	PostageJapan      float64
	SellingPrice      float64
	GrossProfitMargin float64
	ProdUrl           string
	VoucherDataId     int
	ProdMstDeleteFlag bool
}

type ConfigData struct {
	DataId            int
	GrossProfitMargin float64
	Currency          string
}

// 為替レート
type ExchangeRateData struct {
	DataId       int
	Date         string
	Base, Symbol string
	Rate         float64
}

var DbConnection *sql.DB

func init() {
	cfg, _ := ini.Load("config.ini")
	cfg.MapTo(&Config)
}

func main() {
	http.HandleFunc("/home/", homeHandler)

	http.HandleFunc("/product_mst_edit/", productMstEditHandler)
	http.HandleFunc("/product_mst_save/", productMstSaveHandler)
	http.HandleFunc("/product_mst_delete/", productMstDeleteHandler)
	http.HandleFunc("/product_mst_revival/", productMstRevivalHandler)

	http.HandleFunc("/voucher_data_edit/", voucherDataEditHandler)
	http.HandleFunc("/voucher_data_save/", voucherDataSaveHandler)
	http.HandleFunc("/voucher_data_delete/", voucherDataDeleteHandler)
	http.HandleFunc("/voucher_data_force_delete/", voucherDataForceDeleteHandler)

	http.HandleFunc("/product_data_edit/", productDataEditHandler)
	http.HandleFunc("/product_data_save/", productDataSaveHandler)
	http.HandleFunc("/product_data_delete/", productDataDeleteHandler)

	http.HandleFunc("/update_gross_profit_margin/", updateGrossProfitMarginHandler)
	http.HandleFunc("/update_currency/", updateCurrencyHandler)

	checkCurrentExchangeRate()

	// サーバー立ち上げ
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// 為替データの更新（なければ更新）
func checkCurrentExchangeRate() {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1) // 何故か直近の日付を指定すると、フォーマットが間違ってると怒られるので一日前のデータを取得する
	yesterday_str := timeutil.Strftime(&yesterday, "%Y-%m-%d")

	eRateDataList := getExchangeRateFromDb()

	hasYesterdayData := linq.From(eRateDataList).AnyWith(
		func(p interface{}) bool {
			return p.(ExchangeRateData).Date == yesterday_str
		},
	)

	hasData := linq.From(eRateDataList).Any()

	// 昨日のデータがなければDBを更新する
	if !hasYesterdayData {
		updateCurrentExchangeRate(yesterday_str, "CNY", "JPY", hasData)
		updateCurrentExchangeRate(yesterday_str, "JPY", "CNY", hasData)
	}
}

func getExchangeRateFromDb() []ExchangeRateData {
	var eRateDataList []ExchangeRateData

	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	cmd := `select * from exchangeRateData`
	rows, _ := DbConnection.Query(cmd)
	defer rows.Close()
	for rows.Next() {
		var rate ExchangeRateData
		rows.Scan(&rate.DataId, &rate.Date, &rate.Base, &rate.Symbol, &rate.Rate)
		eRateDataList = append(eRateDataList, rate)
	}
	err := rows.Err()
	if err != nil {
		fmt.Println(err)
	}
	return eRateDataList
}

func updateCurrentExchangeRate(date, base, symbol string, isUpdate bool) {
	baseUrl, _ := url.Parse("https://api.apilayer.com/exchangerates_data/")
	reference, _ := url.Parse(date)
	endPoint := baseUrl.ResolveReference(reference).String()
	req, _ := http.NewRequest("GET", endPoint, nil)
	req.Header.Set("apikey", "fvu74n7TLvwaqDwx79rt3gLuxvAf2ebY")
	q := req.URL.Query()
	q.Add("base", base)
	q.Add("symbols", symbol)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, _ := client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)

	var ExchangeRate struct {
		Base       string
		Date       string
		Historical bool
		Rates      map[string]float64
		Success    bool
		Timestamp  int
	}
	if err := json.Unmarshal(body, &ExchangeRate); err != nil {
		fmt.Println(err)
	}

	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	if isUpdate {
		cmd := `update exchangeRateData set date = ?, rate = ? where base = ?`
		if _, err := DbConnection.Exec(cmd, date, ExchangeRate.Rates[symbol], base); err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into exchangeRateData (date, base, symbol, rate) values (?, ?, ?, ?)`
		if _, err := DbConnection.Exec(cmd, date, base, symbol, ExchangeRate.Rates[symbol]); err != nil {
			log.Fatalln(err)
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("home.html")
	t.Execute(w, nil)
}

func productMstEditHandler(w http.ResponseWriter, r *http.Request) {
	productMstId := r.FormValue("productMstId")

	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	var productMst ProductMst
	if productMstId != "" {
		cmd := `select * from productMst where productMstId = ?`
		row := DbConnection.QueryRow(cmd, productMstId)
		err := row.Scan(
			&productMst.ProductMstId, &productMst.Name, &productMst.Weight,
			&productMst.Price, &productMst.ProdUrl, &productMst.PostageJapan,
			&productMst.DeleteFlag)
		if err != nil {
			log.Fatalln(err)
		}
	}

	data := make(map[string]interface{})
	data["productMstList"], _ = loadProductMsts()
	data["productMst"] = productMst

	t, _ := template.ParseFiles("product_mst_edit.html")
	t.Execute(w, data)
}

func productMstSaveHandler(w http.ResponseWriter, r *http.Request) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	productMstId := r.FormValue("productMstId")
	name := r.FormValue("name")
	weight := r.FormValue("weight")
	price := r.FormValue("price")
	postageJapan := r.FormValue("postageJapan")
	prodUrl := r.FormValue("prodUrl")

	_productMstId, _ := strconv.Atoi(productMstId)
	if _productMstId > 0 {
		cmd := `update productMst set name = ?, weight = ?, price = ?, postageJapan = ?, prodUrl = ? where productMstId = ?`
		_, err := DbConnection.Exec(cmd, name, weight, price, postageJapan, prodUrl, _productMstId)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into productMst (name, weight, price, postageJapan, prodUrl, deleteFlag) values (?, ?, ?, ?, ?, ?)`
		_, err := DbConnection.Exec(cmd, name, weight, price, postageJapan, prodUrl, false)
		if err != nil {
			log.Fatalln(err)
		}
	}

	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

func productMstDeleteHandler(w http.ResponseWriter, r *http.Request) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	productMstId := r.FormValue("productMstId")
	cmd := `update productMst set deleteFlag = ? where productMstId = ?`
	_, err := DbConnection.Exec(cmd, true, productMstId)
	if err != nil {
		log.Fatalln(err)
	}
	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

func productMstRevivalHandler(w http.ResponseWriter, r *http.Request) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	productMstId := r.FormValue("productMstId")
	cmd := `update productMst set deleteFlag = ? where productMstId = ?`
	_, err := DbConnection.Exec(cmd, false, productMstId)
	if err != nil {
		log.Fatalln(err)
	}
	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

// 伝票入力
func voucherDataEditHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataList, _ := loadVoucherDatas()

	var vDataList []VoucherData2
	for _, v := range voucherDataList {
		var vData VoucherData2
		b, _ := json.Marshal(v)
		if err := json.Unmarshal(b, &vData); err != nil {
			log.Fatalln(err)
		}
		vDataList = append(vDataList, vData)
	}

	linq.From(vDataList).Where(func(i interface{}) bool {
		return !i.(VoucherData2).DeleteFlag
	}).ToSlice(&vDataList)

	data := make(map[string]interface{})
	data["shippingModeList"] = Config.ShippingMode
	data["shippingMethodList"] = Config.ShippingMethod
	data["statusList"] = Config.Status
	data["voucherDataList"] = vDataList
	t, _ := template.ParseFiles("voucher_data_edit.html")
	t.Execute(w, data)
}

func (v *VoucherData2) UnmarshalJSON(b []byte) error {
	var vData VoucherData
	err := json.Unmarshal(b, &vData)
	if err != nil {
		log.Fatalln(err)
	}

	v.VoucherDataId = vData.VoucherDataId
	v.OrderId = vData.OrderId
	v.OrderTime = vData.OrderTime
	v.ShippingMode = vData.ShippingMode
	v.ShippingModeName = Config.ShippingMode[vData.ShippingMode]
	v.ShippingMethod = vData.ShippingMethod
	v.ShippingMethodName = Config.ShippingMethod[vData.ShippingMethod]
	v.DeliveryId = vData.DeliveryId
	v.Weight = vData.Weight
	v.Cost = vData.Cost
	v.Status = vData.Status
	v.StatusName = Config.Status[vData.Status]
	v.DeleteFlag = vData.DeleteFlag

	return err
}

// 伝票入力
func voucherDataSaveHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId := r.FormValue("voucherDataId")
	orderId := r.FormValue("orderId")
	orderTime := r.FormValue("orderTime")
	shippingMode := r.FormValue("shippingMode")
	shippingMethod := r.FormValue("shippingMethod")
	deliveryId := r.FormValue("deliveryId")
	weight := r.FormValue("weight")
	cost := r.FormValue("cost")
	status := r.FormValue("status")
	_voucherDataId, _ := strconv.Atoi(voucherDataId)

	if len(orderId) <= 0 {
		orderId = "-"
	}

	if len(orderTime) <= 0 {
		orderTime = time.Now().Format("2006-01-02")
	}

	if len(deliveryId) <= 0 {
		deliveryId = "-"
	}

	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	if _voucherDataId > 0 {
		cmd := `update voucherData 
		set orderId = ?, orderTime = ?, shippingMode = ?, shippingMethod = ?, 
		deliveryId = ?, weight = ?, cost = ?, status = ?
		where voucherDataId = ?`
		_, err := DbConnection.Exec(cmd, orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status, voucherDataId)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into voucherData (orderId,orderTime,shippingMode,shippingMethod,deliveryId,weight,cost,status,deleteFlag) values (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := DbConnection.Exec(cmd, orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status, false)
		if err != nil {
			log.Fatalln(err)
		}
	}
	http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
}

// 伝票データ削除
func voucherDataDeleteHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	productDatas, _ := loadProductDatas()
	pData := linq.From(productDatas).Where(func(i interface{}) bool {
		return i.(ProductData).VoucherDataId == voucherDataId
	}).Select(func(i interface{}) interface{} {
		return i.(ProductData)
	})
	var hasProductData bool
	if pData.Count() > 0 {
		hasProductData = true
	} else {
		hasProductData = false

		deleteVoucherDataById(voucherDataId)
		http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
	}
	jsonData := fmt.Sprintf(`{"hasProductData":%t}`, hasProductData)

	json.NewEncoder(w).Encode(&jsonData)
}

// 伝票データの強制削除
func voucherDataForceDeleteHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	deleteVoucherDataById(voucherDataId)
	http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
}

func deleteVoucherDataById(voucherDataId int) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	cmd := `update voucherData set deleteFlag = true where voucherDataId = ?`
	DbConnection.Exec(cmd, voucherDataId)
}

func loadVoucherDatas() ([]VoucherData, error) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	cmd := `select * from voucherData`
	rows, _ := DbConnection.Query(cmd)
	defer rows.Close()
	var vouchers []VoucherData
	for rows.Next() {
		var v VoucherData
		rows.Scan(&v.VoucherDataId, &v.OrderId, &v.OrderTime,
			&v.ShippingMode, &v.ShippingMethod, &v.DeliveryId,
			&v.Weight, &v.Cost, &v.Status, &v.DeleteFlag)
		vouchers = append(vouchers, v)
	}
	err := rows.Err()
	if err != nil {
		return nil, err
	}
	return vouchers, nil
}

func productDataEditHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	if voucherDataId <= 0 {
		http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
	}

	var pData ProductData2
	var pDataList []ProductData2
	productDataList, _ := loadProductDatas()

	linq.From(productDataList).WhereT(func(p ProductData) bool {
		return p.VoucherDataId == voucherDataId
	}).ToSlice(&productDataList)

	for _, p := range productDataList {
		v, _ := json.Marshal(p)
		err := json.Unmarshal(v, &pData)
		if err != nil {
			fmt.Println(err)
		}
		pDataList = append(pDataList, pData)
	}

	// configData取得
	configData := getConfigData()

	// 伝票データ取得
	voucherData := getVoucherDataById(voucherDataId)
	voucherDataByte, err := json.Marshal(voucherData)
	if err != nil {
		log.Fatalln(err)
	}
	var voucherData2 VoucherData2
	err = json.Unmarshal(voucherDataByte, &voucherData2)
	if err != nil {
		log.Fatalln(err)
	}

	data := make(map[string]interface{})
	data["voucherData"] = voucherData2
	data["productMstList"], _ = loadProductMsts()
	data["productDataList"] = pDataList
	data["configData"] = configData

	t, _ := template.ParseFiles("product_data_edit.html")
	t.Execute(w, data)
}

func getVoucherDataById(voucherDataId int) VoucherData {
	voucherDataList, _ := loadVoucherDatas()
	voucherData := linq.From(voucherDataList).FirstWith(func(i interface{}) bool {
		return i.(VoucherData).VoucherDataId == voucherDataId
	})
	return voucherData.(VoucherData)
}

func (p *ProductData2) UnmarshalJSON(b []byte) error {
	var pData ProductData
	err := json.Unmarshal(b, &pData)
	if err != nil {
		fmt.Println(err)
	}

	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	var name string
	var price float64
	var prodUrl string
	var postageJapan float64
	var weight float64
	var pMst ProductMst
	cmd := `select * from productMst where productMstId = ?`
	row := DbConnection.QueryRow(cmd, pData.ProductMstId)
	err = row.Scan(&pMst.ProductMstId, &pMst.Name, &pMst.Weight, &pMst.Price, &pMst.ProdUrl, &pMst.PostageJapan, &pMst.DeleteFlag)
	if err != nil {
		fmt.Printf("error is = %v", err)
		name = "マスターが見つかりません！"
		price = 0
		prodUrl = ""
		postageJapan = 0
		weight = 0
	} else {
		name = pMst.Name
		price = pMst.Price
		prodUrl = pMst.ProdUrl
		postageJapan = pMst.PostageJapan
		weight = pMst.Weight
	}
	p.ProdMstDeleteFlag = pMst.DeleteFlag

	// configData取得
	configData := getConfigData()

	p.ProductDataId = pData.ProductDataId
	p.ProductMstId = pData.ProductMstId
	p.Name = name
	p.Count = pData.Count
	p.Weight = pData.Weight
	p.Price = pData.Price
	p.PostageChina = pData.PostageChina
	p.ProdUrl = prodUrl

	// 粗利率
	var grossProfitMargin float64
	if pData.GrossProfitMargin > 0 {
		grossProfitMargin = pData.GrossProfitMargin
	} else {
		grossProfitMargin = configData.GrossProfitMargin
	}
	p.GrossProfitMargin = grossProfitMargin

	eRateDataList := getExchangeRateFromDb()
	eRateData := linq.From(eRateDataList).FirstWithT(
		func(d ExchangeRateData) bool {
			return d.Symbol == configData.Currency
		},
	)

	// 販売価格（元）
	var sellingPrice = price / (1 - grossProfitMargin/100)

	// 中国送料（元）
	var postageChina = pData.PostageChina / float64(pData.Count)

	voucherData := getVoucherDataById(pData.VoucherDataId)

	// 国際送料
	postageInternational := 0.0
	if voucherData.Weight > 0 && voucherData.Cost > 0 {
		postageInternational = weight / voucherData.Weight * float64(voucherData.Cost)
	}

	if configData.Currency == "JPY" {
		// 円表示
		sellingPrice *= eRateData.(ExchangeRateData).Rate
		postageChina *= eRateData.(ExchangeRateData).Rate
		postageInternational *= eRateData.(ExchangeRateData).Rate
	} else {
		// 元表示
		postageJapan *= eRateData.(ExchangeRateData).Rate
		postageJapan, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", postageJapan), 64)
	}
	p.PostageJapan = postageJapan

	// 一個当たりの販売価格 = 原価 / 原価率 + 中国送料（1個あたり）+ 日本送料 + 国際送料（商品ごと）+ 販売手数料（10%）
	sellingPrice = sellingPrice + postageChina + postageJapan + postageInternational
	sellingPrice += sellingPrice * 0.1

	sellingPrice, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", sellingPrice), 64)

	p.SellingPrice = sellingPrice

	return err
}

func productDataSaveHandler(w http.ResponseWriter, r *http.Request) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	voucherDataId := r.FormValue("voucherDataId")
	productDataId := r.FormValue("productDataId")
	productMstId := r.FormValue("productMstId")
	count := r.FormValue("count")
	weight := r.FormValue("weight")
	price := r.FormValue("price")
	postageChina := r.FormValue("postageChina")
	grossProfitMargin := r.FormValue("grossProfitMargin")

	_productDataId, _ := strconv.Atoi(productDataId)
	if _productDataId > 0 {
		cmd := `update productData 
		set productMstId = ?, count = ?, weight = ?, price = ?, 
		postageChina = ?, grossProfitMargin = ?
		where productDataId = ?`
		_, err := DbConnection.Exec(cmd,
			productMstId, count, weight, price,
			postageChina, grossProfitMargin,
			_productDataId)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into productData (productMstId, count, weight, price, postageChina, grossProfitMargin, voucherDataId) values (?, ?, ?, ?, ?, ?, ?)`
		_, err := DbConnection.Exec(cmd, productMstId, count, weight, price, postageChina, grossProfitMargin, voucherDataId)
		if err != nil {
			log.Fatalln(err)
		}
	}
	http.Redirect(w, r, "/product_data_edit/?voucherDataId="+voucherDataId, http.StatusFound)
}

func productDataDeleteHandler(w http.ResponseWriter, r *http.Request) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	voucherDataId := r.FormValue("voucherDataId")
	productDataId := r.FormValue("productDataId")
	_productDataId, _ := strconv.Atoi(productDataId)
	if _productDataId > 0 {
		cmd := `delete from productData where productDataId = ?`
		_, err := DbConnection.Exec(cmd, _productDataId)
		if err != nil {
			log.Fatalln(err)
		}
	}
	http.Redirect(w, r, "/product_data_edit/?voucherDataId="+voucherDataId, http.StatusFound)
}

func updateGrossProfitMarginHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId := r.FormValue("voucherDataId")
	grossProfitMargin := r.FormValue("grossProfitMargin")
	_grossProfitMargin, _ := strconv.Atoi(grossProfitMargin)
	if _grossProfitMargin > 0 {
		DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
		defer DbConnection.Close()
		cmd := `update configData set grossProfitMargin = ? where dataId = ?`
		_, err := DbConnection.Exec(cmd, _grossProfitMargin, 1)
		if err != nil {
			log.Fatalln(err)
		}
	}
	http.Redirect(w, r, "/product_data_edit/?voucherDataId="+voucherDataId, http.StatusFound)
}

func updateCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId := r.FormValue("voucherDataId")
	currency := r.FormValue("currency")
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	cmd := `update configData set currency = ? where dataId = ?`
	_, err := DbConnection.Exec(cmd, currency, 1)
	if err != nil {
		log.Fatalln(err)
	}
	http.Redirect(w, r, "/product_data_edit/?voucherDataId="+voucherDataId, http.StatusFound)
}

func loadProductDatas() ([]ProductData, error) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	cmd := `select * from productData`
	rows, _ := DbConnection.Query(cmd)
	defer rows.Close()
	var products []ProductData
	for rows.Next() {
		var p ProductData
		if err := rows.Scan(&p.ProductDataId, &p.ProductMstId, &p.Count, &p.Weight, &p.Price, &p.PostageChina, &p.GrossProfitMargin, &p.VoucherDataId); err != nil {
			log.Fatalln(err)
		}
		products = append(products, p)
	}
	err := rows.Err()
	if err != nil {
		return nil, err
	}
	return products, nil
}

func loadProductMsts() ([]ProductMst, error) {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()
	cmd := `select * from productMst`
	rows, _ := DbConnection.Query(cmd)
	defer rows.Close()
	var products []ProductMst
	for rows.Next() {
		var p ProductMst
		rows.Scan(&p.ProductMstId, &p.Name, &p.Weight, &p.Price, &p.ProdUrl, &p.PostageJapan, &p.DeleteFlag)
		products = append(products, p)
	}
	err := rows.Err()
	if err != nil {
		return nil, err
	}
	return products, nil
}

func getConfigData() ConfigData {
	DbConnection, _ := sql.Open("sqlite3", "./sellManagement.sql")
	defer DbConnection.Close()

	var configData ConfigData
	cmd := `select * from configData where dataId = ?`
	row := DbConnection.QueryRow(cmd, 1)
	err := row.Scan(&configData.DataId, &configData.GrossProfitMargin, &configData.Currency)
	if err != nil {
		log.Fatalln(err)
	}
	return configData
}
