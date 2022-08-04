package main

import (
	"SellManagement/app/models"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ahmetalpbalkan/go-linq"
	"github.com/jmoiron/sqlx"
	"github.com/leekchan/timeutil"
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
	ProductMstId int     `db:"productMstId"`
	Name         string  `db:"name"`
	Weight       float64 `db:"weight"`
	Price        float64 `db:"price"`
	ProdUrl      string  `db:"prodUrl"`
	PostageJapan float64 `db:"postageJapan"`
	DeleteFlag   bool    `db:"deleteFlag"`
}

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
	GrossProfitMargin float64
	ProdUrl           string
	VoucherDataId     int
	ProdMstDeleteFlag bool
}

type ProductDetailData struct {
	ProductDetailDataId int     `db:"productDetailDataId"`
	ProductDataId       int     `db:"productDataId"`
	GrossProfitMargin   float64 `db:"grossProfitMargin"`
	PurchaseDate        string  `db:"purchaseDate"`
	SalesDate           string  `db:"salesDate"`
}

type ProductDetailData2 struct {
	ProductDetailDataId int
	ProductDataId       int
	GrossProfitMargin   float64
	PurchaseDate        string
	SalesDate           string
	ProductMstId        int
	Name                string
	SellingPrice        float64
	ProdUrl             string
}

type InventoryData struct {
	ProductMstId  int
	Name          string
	ProdUrl       string
	PurchaseCount int
	StockCount    int
	SoldCount     int
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

const view_prefix string = "app/views/"

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

	http.HandleFunc("/product_detail_data_edit/", productDetailDataEditHandler)
	http.HandleFunc("/product_detail_data_update/", productDetailDataUpdateHandler)
	http.HandleFunc("/product_detail_salesDate_update/", productDetailSalesDateUpdateHandler)

	http.HandleFunc("/product_selling_edit/", productSellingEditHandler)
	http.HandleFunc("/inventory_list_view/", inventoryListViewHandler)

	http.HandleFunc("/config_edit/", configEditHandler)
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

	cmd := `select * from exchangeRateData`
	rows, _ := models.DbConnection.Query(cmd)
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

	if isUpdate {
		cmd := `update exchangeRateData set date = ?, rate = ? where base = ?`
		if _, err := models.DbConnection.Exec(cmd, date, ExchangeRate.Rates[symbol], base); err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into exchangeRateData (date, base, symbol, rate) values (?, ?, ?, ?)`
		if _, err := models.DbConnection.Exec(cmd, date, base, symbol, ExchangeRate.Rates[symbol]); err != nil {
			log.Fatalln(err)
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles(view_prefix + "home.html")
	t.Execute(w, nil)
}

func productMstEditHandler(w http.ResponseWriter, r *http.Request) {
	productMstId := r.FormValue("productMstId")

	var productMst ProductMst
	if productMstId != "" {
		cmd := `select * from productMst where productMstId = ?`
		row := models.DbConnection.QueryRow(cmd, productMstId)
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

	t, _ := template.ParseFiles(view_prefix + "product_mst_edit.html")
	t.Execute(w, data)
}

func productMstSaveHandler(w http.ResponseWriter, r *http.Request) {

	productMstId := r.FormValue("productMstId")
	name := r.FormValue("name")
	weight := r.FormValue("weight")
	price := r.FormValue("price")
	postageJapan := r.FormValue("postageJapan")
	prodUrl := r.FormValue("prodUrl")

	_productMstId, _ := strconv.Atoi(productMstId)
	if _productMstId > 0 {
		cmd := `update productMst set name = ?, weight = ?, price = ?, postageJapan = ?, prodUrl = ? where productMstId = ?`
		_, err := models.DbConnection.Exec(cmd, name, weight, price, postageJapan, prodUrl, _productMstId)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into productMst (name, weight, price, postageJapan, prodUrl, deleteFlag) values (?, ?, ?, ?, ?, ?)`
		_, err := models.DbConnection.Exec(cmd, name, weight, price, postageJapan, prodUrl, false)
		if err != nil {
			log.Fatalln(err)
		}
	}

	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

func productMstDeleteHandler(w http.ResponseWriter, r *http.Request) {

	productMstId := r.FormValue("productMstId")
	cmd := `update productMst set deleteFlag = ? where productMstId = ?`
	_, err := models.DbConnection.Exec(cmd, true, productMstId)
	if err != nil {
		log.Fatalln(err)
	}
	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

func productMstRevivalHandler(w http.ResponseWriter, r *http.Request) {

	productMstId := r.FormValue("productMstId")
	cmd := `update productMst set deleteFlag = ? where productMstId = ?`
	_, err := models.DbConnection.Exec(cmd, false, productMstId)
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
	t, _ := template.ParseFiles(view_prefix + "voucher_data_edit.html")
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

	if _voucherDataId > 0 {
		cmd := `update voucherData 
		set orderId = ?, orderTime = ?, shippingMode = ?, shippingMethod = ?, 
		deliveryId = ?, weight = ?, cost = ?, status = ?
		where voucherDataId = ?`
		_, err := models.DbConnection.Exec(cmd, orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status, voucherDataId)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into voucherData (orderId,orderTime,shippingMode,shippingMethod,deliveryId,weight,cost,status,deleteFlag) values (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := models.DbConnection.Exec(cmd, orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status, false)
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

	cmd := `update voucherData set deleteFlag = true where voucherDataId = ?`
	models.DbConnection.Exec(cmd, voucherDataId)
}

func loadVoucherDatas() ([]VoucherData, error) {

	cmd := `select * from voucherData`
	rows, _ := models.DbConnection.Query(cmd)
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

	// 伝票データ取得
	voucherData2 := getVoucherData2ById(voucherDataId)

	data := make(map[string]interface{})
	data["voucherData"] = voucherData2
	data["productMstList"], _ = loadProductMsts()
	data["productDataList"] = pDataList

	t, _ := template.ParseFiles(view_prefix + "product_data_edit.html")
	t.Execute(w, data)
}

func getVoucherDataById(voucherDataId int) VoucherData {
	voucherDataList, _ := loadVoucherDatas()
	voucherData := linq.From(voucherDataList).FirstWith(func(i interface{}) bool {
		return i.(VoucherData).VoucherDataId == voucherDataId
	})
	return voucherData.(VoucherData)
}

func getVoucherData2ById(voucherDataId int) VoucherData2 {
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
	return voucherData2
}

func (p *ProductData2) UnmarshalJSON(b []byte) error {
	var pData ProductData
	err := json.Unmarshal(b, &pData)
	if err != nil {
		fmt.Println(err)
	}

	var name string
	var prodUrl string
	var postageJapan float64
	var weight float64
	var pMst ProductMst
	cmd := `select * from productMst where productMstId = ?`
	row := models.DbConnection.QueryRow(cmd, pData.ProductMstId)
	err = row.Scan(&pMst.ProductMstId, &pMst.Name, &pMst.Weight, &pMst.Price, &pMst.ProdUrl, &pMst.PostageJapan, &pMst.DeleteFlag)
	if err != nil {
		fmt.Printf("error is = %v", err)
		name = "マスターが見つかりません！"
		prodUrl = ""
		postageJapan = 0
		weight = 0
	} else {
		name = pMst.Name
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
		postageChina *= eRateData.(ExchangeRateData).Rate
		postageInternational *= eRateData.(ExchangeRateData).Rate
	} else {
		// 元表示
		postageJapan *= eRateData.(ExchangeRateData).Rate
		postageJapan, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", postageJapan), 64)
	}
	p.PostageJapan = postageJapan

	return err
}

func productDataSaveHandler(w http.ResponseWriter, r *http.Request) {

	voucherDataId := r.FormValue("voucherDataId")
	productDataId, _ := strconv.Atoi(r.FormValue("productDataId"))
	productMstId := r.FormValue("productMstId")
	count, _ := strconv.Atoi(r.FormValue("count"))
	weight := r.FormValue("weight")
	price := r.FormValue("price")
	postageChina := r.FormValue("postageChina")
	grossProfitMargin := r.FormValue("grossProfitMargin")

	if productDataId > 0 {
		cmd := `update productData 
		set productMstId = ?, count = ?, weight = ?, price = ?, 
		postageChina = ?, grossProfitMargin = ?
		where productDataId = ?`
		_, err := models.DbConnection.Exec(cmd,
			productMstId, count, weight, price,
			postageChina, grossProfitMargin,
			productDataId)
		if err != nil {
			log.Fatalln(err)
		}

		cmd = `update productDetailData set grossProfitMargin = ? where productDataId = ?`
		_, err = models.DbConnection.Exec(cmd, grossProfitMargin, productDataId)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		cmd := `insert into productData (productMstId, count, weight, price, postageChina, grossProfitMargin, voucherDataId) values (?, ?, ?, ?, ?, ?, ?)`
		res, err := models.DbConnection.Exec(cmd, productMstId, count, weight, price, postageChina, grossProfitMargin, voucherDataId)
		if err != nil {
			log.Fatalln(err)
		}

		if lastId, err := res.LastInsertId(); err != nil {
			log.Fatalln(err)
		} else {
			// 詳細データ
			for i := 0; i < count; i++ {
				cmd := `insert into productDetailData (productDataId, grossProfitMargin) values (?, ?)`
				_, err := models.DbConnection.Exec(cmd, lastId, grossProfitMargin)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
	}
	http.Redirect(w, r, "/product_data_edit/?voucherDataId="+voucherDataId, http.StatusFound)
}

func productDataDeleteHandler(w http.ResponseWriter, r *http.Request) {

	voucherDataId := r.FormValue("voucherDataId")
	productDataId := r.FormValue("productDataId")
	_productDataId, _ := strconv.Atoi(productDataId)
	if _productDataId > 0 {
		cmd := `delete from productData where productDataId = ?`
		_, err := models.DbConnection.Exec(cmd, _productDataId)
		if err != nil {
			log.Fatalln(err)
		}

		cmd = `delete from productDetailData where productDataId = ?`
		_, err = models.DbConnection.Exec(cmd, _productDataId)
		if err != nil {
			log.Fatalln(err)
		}
	}
	http.Redirect(w, r, "/product_data_edit/?voucherDataId="+voucherDataId, http.StatusFound)
}

func productDetailDataEditHandler(w http.ResponseWriter, r *http.Request) {
	productDataId, _ := strconv.Atoi(r.FormValue("productDataId"))

	// productDataの取得
	productDataList, _ := loadProductDatas()
	productData := linq.From(productDataList).FirstWith(func(i interface{}) bool {
		return i.(ProductData).ProductDataId == productDataId
	}).(ProductData)

	// productMstの取得
	productMst := getProductMstById(productData.ProductMstId)

	// productDetailDataの取得
	details := getProductDetailDataByProductDataId(productDataId)

	var details2 []ProductDetailData2
	for _, detail := range details {
		// productDetailData2へ変換
		detail2 := convertToProductDetailData2(detail, productData, productMst)
		details2 = append(details2, detail2)
	}

	voucherData2 := getVoucherData2ById(productData.VoucherDataId)

	data := make(map[string]interface{})
	data["voucherData"] = voucherData2
	data["productDetailDataList"] = details2

	t, _ := template.ParseFiles(view_prefix + "product_detail_data_edit.html")
	t.Execute(w, data)
}

func convertToProductDetailData2(detail ProductDetailData, productData ProductData, productMst ProductMst) ProductDetailData2 {
	detail2 := ProductDetailData2{
		ProductDetailDataId: detail.ProductDetailDataId,
		ProductDataId:       detail.ProductDataId,
		GrossProfitMargin:   detail.GrossProfitMargin,
		PurchaseDate:        detail.PurchaseDate,
		SalesDate:           detail.SalesDate,
		ProductMstId:        productMst.ProductMstId,
		Name:                productMst.Name,
		SellingPrice:        getSellingPrice(detail, productData, productMst),
		ProdUrl:             productMst.ProdUrl,
	}
	return detail2
}

func productDetailDataUpdateHandler(w http.ResponseWriter, r *http.Request) {
	productDetailDataId, _ := strconv.Atoi(r.FormValue("productDetailDataId"))
	productDataId := r.FormValue("productDataId")
	grossProfitMargin := r.FormValue("grossProfitMargin")

	cmd := `update productDetailData set grossProfitMargin = ? where productDetailDataId = ?`
	_, err := models.DbConnection.Exec(cmd, grossProfitMargin, productDetailDataId)
	if err != nil {
		log.Fatalln(err)
	}
	http.Redirect(w, r, "/product_detail_data_edit/?productDataId="+productDataId, http.StatusFound)
}

func productDetailSalesDateUpdateHandler(w http.ResponseWriter, r *http.Request) {
	productDetailDataIdList := r.FormValue("productDetailDataIdList")
	productDetailDataIds := strings.Split(productDetailDataIdList, ",")

	flag := r.FormValue("flag")

	_now := "0"
	if flag == "sold" {
		now := time.Now()
		_now = timeutil.Strftime(&now, "%Y-%m-%d %H:%M:%S")
	}

	db, _ := sqlx.Open("sqlite3", "./sellManagement.sql")
	defer db.Close()
	cmd := `update productDetailData set salesDate = ? where productDetailDataId in (?)`
	query, args, err := sqlx.In(cmd, _now, productDetailDataIds)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = db.Exec(query, args...)
	if err != nil {
		log.Fatalln(err)
	}
	http.Redirect(w, r, "/product_selling_edit/", http.StatusFound)
}

func productSellingEditHandler(w http.ResponseWriter, r *http.Request) {

	cmd := `select * from productDetailData`
	rows, _ := models.DbConnection.Query(cmd)
	defer rows.Close()
	var pDetials []ProductDetailData
	for rows.Next() {
		var p ProductDetailData
		rows.Scan(&p.ProductDetailDataId, &p.ProductDataId, &p.GrossProfitMargin,
			&p.PurchaseDate, &p.SalesDate)
		pDetials = append(pDetials, p)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln(err)
	}

	var productDataIds []int
	linq.From(pDetials).Select(func(i interface{}) interface{} {
		return i.(ProductDetailData).ProductDataId
	}).Distinct().ToSlice(&productDataIds)

	// 対象のproductDataList取得
	productDataList := getProductDataListByIds(productDataIds)

	var productMstIds []int
	linq.From(productDataList).Select(func(i interface{}) interface{} {
		return i.(ProductData).ProductMstId
	}).Distinct().ToSlice(&productMstIds)

	// 対象のproductMstList取得
	productMstList := getProductMstListByIds(productMstIds)

	var details2 []ProductDetailData2
	for _, detail := range pDetials {
		pData := linq.From(productDataList).FirstWith(func(i interface{}) bool { return i.(ProductData).ProductDataId == detail.ProductDataId }).(ProductData)
		pMst := linq.From(productMstList).FirstWith(func(i interface{}) bool { return i.(ProductMst).ProductMstId == pData.ProductMstId }).(ProductMst)
		detail2 := convertToProductDetailData2(detail, pData, pMst)
		details2 = append(details2, detail2)
	}

	data := make(map[string]interface{})
	data["productDetailDataList"] = details2
	t, _ := template.ParseFiles(view_prefix + "product_selling_edit.html")
	t.Execute(w, data)
}

func inventoryListViewHandler(w http.ResponseWriter, r *http.Request) {
	// productMstList取得
	productMstList, err := loadProductMsts()
	if err != nil {
		log.Fatalln(err)
	}

	// productDataList取得
	productDataList, err := loadProductDatas()
	if err != nil {
		log.Fatalln(err)
	}

	productDetailDataList := loadProductDetailDatas()

	var inventoryList []InventoryData
	for _, pMst := range productMstList {
		purchaseCount := 0
		stockCount := 0
		soldCount := 0
		var pDatas []ProductData
		linq.From(productDataList).Where(func(i interface{}) bool { return i.(ProductData).ProductMstId == pMst.ProductMstId }).ToSlice(&pDatas)
		for _, pData := range pDatas {
			pDetailDataQuery := linq.From(productDetailDataList).Where(func(i interface{}) bool { return i.(ProductDetailData).ProductDataId == pData.ProductDataId })
			purchaseCount += pDetailDataQuery.Count()
			stockCount += pDetailDataQuery.Where(func(i interface{}) bool { return i.(ProductDetailData).SalesDate == "0" }).Count()
			soldCount += pDetailDataQuery.Where(func(i interface{}) bool { return i.(ProductDetailData).SalesDate != "0" }).Count()
		}

		inventory := InventoryData{
			pMst.ProductMstId,
			pMst.Name,
			pMst.ProdUrl,
			purchaseCount,
			stockCount,
			soldCount,
		}
		inventoryList = append(inventoryList, inventory)
	}

	data := make(map[string]interface{})
	data["inventoryList"] = inventoryList
	t, _ := template.ParseFiles(view_prefix + "inventory_list_view.html")
	t.Execute(w, data)
}

// 販売価格の計算
func getSellingPrice(pDetail ProductDetailData, pData ProductData, pMst ProductMst) float64 {
	// configData取得
	configData := getConfigData()

	// 粗利率
	var grossProfitMargin float64
	if pDetail.GrossProfitMargin > 0 {
		grossProfitMargin = pDetail.GrossProfitMargin
	} else if pData.GrossProfitMargin > 0 {
		grossProfitMargin = pData.GrossProfitMargin
	} else {
		grossProfitMargin = configData.GrossProfitMargin
	}

	eRateDataList := getExchangeRateFromDb()
	eRateData := linq.From(eRateDataList).FirstWithT(
		func(d ExchangeRateData) bool {
			return d.Symbol == configData.Currency
		},
	)

	// 販売価格（元）
	var sellingPrice = pMst.Price / (1 - grossProfitMargin/100)

	// 中国送料（元）
	var postageChina = pData.PostageChina / float64(pData.Count)

	// 日本送料（円）
	postageJapan := pMst.PostageJapan

	voucherData := getVoucherDataById(pData.VoucherDataId)

	// 国際送料
	postageInternational := 0.0
	if voucherData.Weight > 0 && voucherData.Cost > 0 {
		postageInternational = pMst.Weight / voucherData.Weight * float64(voucherData.Cost)
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

	// 一個当たりの販売価格 = 原価 / 原価率 + 中国送料（1個あたり）+ 日本送料 + 国際送料（商品ごと）+ 販売手数料（10%）
	sellingPrice = sellingPrice + postageChina + postageJapan + postageInternational
	sellingPrice += sellingPrice * 0.1

	sellingPrice, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", sellingPrice), 64)
	return sellingPrice
}

func getProductDetailDataByProductDataId(productDataId int) []ProductDetailData {

	cmd := `select * from productDetailData where productDataId = ?`
	rows, _ := models.DbConnection.Query(cmd, productDataId)
	defer rows.Close()
	var details []ProductDetailData
	for rows.Next() {
		var d ProductDetailData
		if err := rows.Scan(&d.ProductDetailDataId, &d.ProductDataId, &d.GrossProfitMargin, &d.PurchaseDate, &d.SalesDate); err != nil {
			log.Fatalln(err)
		}
		details = append(details, d)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln(err)
	}
	return details
}

func configEditHandler(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})
	data["configData"] = getConfigData()
	t, _ := template.ParseFiles(view_prefix + "config_edit.html")
	t.Execute(w, data)
}

func updateGrossProfitMarginHandler(w http.ResponseWriter, r *http.Request) {
	grossProfitMargin := r.FormValue("grossProfitMargin")
	_grossProfitMargin, _ := strconv.Atoi(grossProfitMargin)
	if _grossProfitMargin > 0 {

		cmd := `update configData set grossProfitMargin = ? where dataId = ?`
		_, err := models.DbConnection.Exec(cmd, _grossProfitMargin, 1)
		if err != nil {
			log.Fatalln(err)
		}
	}
	http.Redirect(w, r, "/config_edit/", http.StatusFound)
}

func updateCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	currency := r.FormValue("currency")

	cmd := `update configData set currency = ? where dataId = ?`
	_, err := models.DbConnection.Exec(cmd, currency, 1)
	if err != nil {
		log.Fatalln(err)
	}
	http.Redirect(w, r, "/config_edit/", http.StatusFound)
}

func loadProductDatas() ([]ProductData, error) {

	cmd := `select * from productData`
	rows, _ := models.DbConnection.Query(cmd)
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

func loadProductDetailDatas() []ProductDetailData {
	db, _ := sqlx.Open("sqlite3", "./sellManagement.sql")
	defer db.Close()
	cmd := `select * from productDetailData`
	rows, _ := db.Queryx(cmd)
	defer rows.Close()
	var details []ProductDetailData
	for rows.Next() {
		var p ProductDetailData
		if err := rows.StructScan(&p); err != nil {
			log.Fatalln(err)
		}
		details = append(details, p)
	}
	err := rows.Err()
	if err != nil {
		log.Fatalln(err)
	}
	return details
}

func loadProductMsts() ([]ProductMst, error) {

	cmd := `select * from productMst`
	rows, _ := models.DbConnection.Query(cmd)
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

	var configData ConfigData
	cmd := `select * from configData where dataId = ?`
	row := models.DbConnection.QueryRow(cmd, 1)
	err := row.Scan(&configData.DataId, &configData.GrossProfitMargin, &configData.Currency)
	if err != nil {
		log.Fatalln(err)
	}
	return configData
}

func getProductMstById(productMstId int) ProductMst {

	var productMst ProductMst
	cmd := `select * from productMst where productMstId = ?`
	row := models.DbConnection.QueryRow(cmd, productMstId)
	err := row.Scan(
		&productMst.ProductMstId, &productMst.Name, &productMst.Weight,
		&productMst.Price, &productMst.ProdUrl, &productMst.PostageJapan,
		&productMst.DeleteFlag)
	if err != nil {
		log.Fatalln(err)
	}
	return productMst
}

func getProductDataListByIds(ids []int) []ProductData {
	db, _ := sqlx.Open("sqlite3", "./sellManagement.sql")
	defer db.Close()

	query, args, err := sqlx.In(`select * from productData where productDataId in (?)`, ids)
	if err != nil {
		log.Fatalln(err)
	}
	query = db.Rebind(query)
	rows, err := db.Queryx(query, args...)
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

func getProductMstListByIds(ids []int) []ProductMst {
	db, _ := sqlx.Open("sqlite3", "./sellManagement.sql")
	defer db.Close()

	query, args, err := sqlx.In(`select * from productMst where productMstId in (?)`, ids)
	if err != nil {
		log.Fatalln(err)
	}
	query = db.Rebind(query)
	rows, err := db.Queryx(query, args...)
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
