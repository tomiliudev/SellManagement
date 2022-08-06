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
	"github.com/leekchan/timeutil"
	"gopkg.in/ini.v1"
)

type ConfigList struct {
	ShippingMode   []string `ini:"shippingMode"`
	ShippingMethod []string `ini:"shippingMethod"`
	Status         []string `ini:"status"`
}

var Config ConfigList

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
	productMstId, _ := strconv.Atoi(r.FormValue("productMstId"))

	var productMst models.ProductMst
	if productMstId > 0 {
		productMst = models.GetProductMstById(productMstId)
	}

	data := make(map[string]interface{})
	data["productMstList"] = models.GetAllProductMst()
	data["productMst"] = productMst

	t, _ := template.ParseFiles(view_prefix + "product_mst_edit.html")
	t.Execute(w, data)
}

func productMstSaveHandler(w http.ResponseWriter, r *http.Request) {
	productMstId, _ := strconv.Atoi(r.FormValue("productMstId"))
	name := r.FormValue("name")
	weight, _ := strconv.ParseFloat(r.FormValue("weight"), 64)
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	postageJapan, _ := strconv.ParseFloat(r.FormValue("postageJapan"), 64)
	prodUrl := r.FormValue("prodUrl")

	if productMstId > 0 {
		models.UpdateProductMstById(productMstId, name, weight, price, postageJapan, prodUrl)
	} else {
		models.InsertProductMst(name, weight, price, postageJapan, prodUrl)
	}

	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

func productMstDeleteHandler(w http.ResponseWriter, r *http.Request) {
	productMstId, _ := strconv.Atoi(r.FormValue("productMstId"))
	models.UpdateProductMstDeleteFlagById(productMstId, true)
	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

func productMstRevivalHandler(w http.ResponseWriter, r *http.Request) {
	productMstId, _ := strconv.Atoi(r.FormValue("productMstId"))
	models.UpdateProductMstDeleteFlagById(productMstId, false)
	http.Redirect(w, r, "/product_mst_edit/", http.StatusFound)
}

// 伝票入力
func voucherDataEditHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataList := models.GetAllVoucherData()
	fmt.Println(voucherDataList)
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
	var vData models.VoucherData
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
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	orderId := r.FormValue("orderId")
	orderTime := r.FormValue("orderTime")
	shippingMode, _ := strconv.Atoi(r.FormValue("shippingMode"))
	shippingMethod, _ := strconv.Atoi(r.FormValue("shippingMethod"))
	deliveryId := r.FormValue("deliveryId")
	weight, _ := strconv.ParseFloat(r.FormValue("weight"), 64)
	cost, _ := strconv.ParseFloat(r.FormValue("cost"), 64)
	status, _ := strconv.Atoi(r.FormValue("status"))

	if len(orderId) <= 0 {
		orderId = "-"
	}

	if len(orderTime) <= 0 {
		orderTime = time.Now().Format("2006-01-02")
	}

	if len(deliveryId) <= 0 {
		deliveryId = "-"
	}

	if voucherDataId > 0 {
		models.UpdateVoucherData(voucherDataId, orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status)
	} else {
		models.InsertVoucherData(orderId, orderTime, shippingMode, shippingMethod, deliveryId, weight, cost, status)
	}
	http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
}

// 伝票データ削除
func voucherDataDeleteHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	productDatas := models.GetAllProductData()
	pData := linq.From(productDatas).Where(func(i interface{}) bool {
		return i.(models.ProductData).VoucherDataId == voucherDataId
	}).Select(func(i interface{}) interface{} {
		return i.(models.ProductData)
	})
	var hasProductData bool
	if pData.Count() > 0 {
		hasProductData = true
	} else {
		hasProductData = false

		models.DeleteVoucherDataById(voucherDataId)
		http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
	}
	jsonData := fmt.Sprintf(`{"hasProductData":%t}`, hasProductData)

	json.NewEncoder(w).Encode(&jsonData)
}

// 伝票データの強制削除
func voucherDataForceDeleteHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	models.DeleteVoucherDataById(voucherDataId)
	http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
}

func productDataEditHandler(w http.ResponseWriter, r *http.Request) {
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	if voucherDataId <= 0 {
		http.Redirect(w, r, "/voucher_data_edit/", http.StatusFound)
	}

	var pData ProductData2
	var pDataList []ProductData2
	productDataList := models.GetAllProductData()

	linq.From(productDataList).WhereT(func(p models.ProductData) bool {
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
	data["productMstList"] = models.GetAllProductMst()
	data["productDataList"] = pDataList

	t, _ := template.ParseFiles(view_prefix + "product_data_edit.html")
	t.Execute(w, data)
}

func getVoucherData2ById(voucherDataId int) VoucherData2 {
	voucherData := models.GetVoucherDataById(voucherDataId)
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
	var pData models.ProductData
	err := json.Unmarshal(b, &pData)
	if err != nil {
		fmt.Println(err)
	}

	pMst := models.GetProductMstById(pData.ProductMstId)
	name := pMst.Name
	prodUrl := pMst.ProdUrl
	postageJapan := pMst.PostageJapan
	weight := pMst.Weight
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

	voucherData := models.GetVoucherDataById(pData.VoucherDataId)

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
	voucherDataId, _ := strconv.Atoi(r.FormValue("voucherDataId"))
	productDataId, _ := strconv.Atoi(r.FormValue("productDataId"))
	productMstId, _ := strconv.Atoi(r.FormValue("productMstId"))
	count, _ := strconv.Atoi(r.FormValue("count"))
	weight, _ := strconv.ParseFloat(r.FormValue("weight"), 64)
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	postageChina, _ := strconv.ParseFloat(r.FormValue("postageChina"), 64)
	grossProfitMargin, _ := strconv.ParseFloat(r.FormValue("grossProfitMargin"), 64)

	if grossProfitMargin <= 0 {
		grossProfitMargin = getConfigData().GrossProfitMargin
	}

	if productDataId > 0 {
		models.UpdateProductDataById(productDataId, productMstId, count, weight, price, postageChina, grossProfitMargin)
		models.UpdateGrossProfitMarginByProductDataId(productDataId, grossProfitMargin)
	} else {
		lastId := models.InsertProductData(productMstId, voucherDataId, count, weight, price, postageChina, grossProfitMargin)
		if lastId > 0 {
			// 詳細データ
			for i := 0; i < count; i++ {
				models.InsertProductDetailData(lastId, grossProfitMargin)
			}
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/product_data_edit/?voucherDataId=%d", voucherDataId), http.StatusFound)
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
	productDataList := models.GetAllProductData()
	productData := linq.From(productDataList).FirstWith(func(i interface{}) bool {
		return i.(models.ProductData).ProductDataId == productDataId
	}).(models.ProductData)

	// productMstの取得
	productMst := models.GetProductMstById(productData.ProductMstId)

	// productDetailDataの取得
	details := models.GetProductDetailDataByProductDataId(productDataId)

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

func convertToProductDetailData2(detail models.ProductDetailData, productData models.ProductData, productMst models.ProductMst) ProductDetailData2 {
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
	grossProfitMargin, _ := strconv.ParseFloat(r.FormValue("grossProfitMargin"), 64)

	models.UpdateGrossProfitMarginById(productDetailDataId, grossProfitMargin)
	http.Redirect(w, r, "/product_detail_data_edit/?productDataId="+productDataId, http.StatusFound)
}

func productDetailSalesDateUpdateHandler(w http.ResponseWriter, r *http.Request) {
	var productDetailDataIds []int
	linq.From(strings.Split(r.FormValue("productDetailDataIdList"), ",")).Select(func(i interface{}) interface{} {
		id, _ := strconv.Atoi(i.(string))
		return id
	}).ToSlice(&productDetailDataIds)

	flag := r.FormValue("flag")

	_now := "0"
	if flag == "sold" {
		now := time.Now()
		_now = timeutil.Strftime(&now, "%Y-%m-%d %H:%M:%S")
	}

	models.UpdateSalesDateInIds(_now, productDetailDataIds)
	http.Redirect(w, r, "/product_selling_edit/", http.StatusFound)
}

func productSellingEditHandler(w http.ResponseWriter, r *http.Request) {
	pDetials := models.GetAllProductDetailData()

	var productDataIds []int
	linq.From(pDetials).Select(func(i interface{}) interface{} {
		return i.(models.ProductDetailData).ProductDataId
	}).Distinct().ToSlice(&productDataIds)

	// 対象のproductDataList取得
	productDataList := models.GetProductDataListByIds(productDataIds)

	var productMstIds []int
	linq.From(productDataList).Select(func(i interface{}) interface{} {
		return i.(models.ProductData).ProductMstId
	}).Distinct().ToSlice(&productMstIds)

	// 対象のproductMstList取得
	productMstList := models.GetProductMstListByIds(productMstIds)

	var details2 []ProductDetailData2
	for _, detail := range pDetials {
		pData := linq.From(productDataList).FirstWith(func(i interface{}) bool { return i.(models.ProductData).ProductDataId == detail.ProductDataId }).(models.ProductData)
		pMst := linq.From(productMstList).FirstWith(func(i interface{}) bool { return i.(models.ProductMst).ProductMstId == pData.ProductMstId }).(models.ProductMst)
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
	productMstList := models.GetAllProductMst()

	// productDataList取得
	productDataList := models.GetAllProductData()
	productDetailDataList := models.GetAllProductDetailData()

	var inventoryList []InventoryData
	for _, pMst := range productMstList {
		purchaseCount := 0
		stockCount := 0
		soldCount := 0
		var pDatas []models.ProductData
		linq.From(productDataList).Where(func(i interface{}) bool { return i.(models.ProductData).ProductMstId == pMst.ProductMstId }).ToSlice(&pDatas)
		for _, pData := range pDatas {
			pDetailDataQuery := linq.From(productDetailDataList).Where(func(i interface{}) bool { return i.(models.ProductDetailData).ProductDataId == pData.ProductDataId })
			purchaseCount += pDetailDataQuery.Count()
			stockCount += pDetailDataQuery.Where(func(i interface{}) bool { return i.(models.ProductDetailData).SalesDate == "0" }).Count()
			soldCount += pDetailDataQuery.Where(func(i interface{}) bool { return i.(models.ProductDetailData).SalesDate != "0" }).Count()
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
func getSellingPrice(pDetail models.ProductDetailData, pData models.ProductData, pMst models.ProductMst) float64 {
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

	voucherData := models.GetVoucherDataById(pData.VoucherDataId)

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
