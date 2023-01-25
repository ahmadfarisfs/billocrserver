package controllers

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/otiai10/gosseract/v2"
	"github.com/otiai10/marmoset"
)

var (
	imgexp = regexp.MustCompile("^image")
)

// FileUpload ...
func FileUpload(w http.ResponseWriter, r *http.Request) {

	render := marmoset.Render(w, true)

	// Get uploaded file
	r.ParseMultipartForm(32 << 20)
	// upload, h, err := r.FormFile("file")
	upload, _, err := r.FormFile("file")
	if err != nil {
		render.JSON(http.StatusBadRequest, err)
		return
	}
	defer upload.Close()

	// Create physical file
	tempfile, err := ioutil.TempFile("", "ocrserver"+"-")
	if err != nil {
		render.JSON(http.StatusBadRequest, err)
		return
	}
	defer func() {
		tempfile.Close()
		os.Remove(tempfile.Name())
	}()

	// Make uploaded physical
	if _, err = io.Copy(tempfile, upload); err != nil {
		render.JSON(http.StatusInternalServerError, err)
		return
	}

	client := gosseract.NewClient()
	defer client.Close()

	client.SetImage(tempfile.Name())
	client.Languages = []string{"eng"}
	if langs := r.FormValue("languages"); langs != "" {
		client.Languages = strings.Split(langs, ",")
	}
	if whitelist := r.FormValue("whitelist"); whitelist != "" {
		client.SetWhitelist(whitelist)
	}

	var out string
	switch r.FormValue("format") {
	case "hocr":
		out, err = client.HOCRText()
		render.EscapeHTML = false
	default:
		out, err = client.Text()
	}
	if err != nil {
		render.JSON(http.StatusBadRequest, err)
		return
	}

	render.JSON(http.StatusOK, map[string]interface{}{
		"result":  strings.Trim(out, r.FormValue("trim")),
		"version": version,
	})
}

type OrderItem struct {
	Name     string `json:"name"`
	Qty      int    `json:"qty"`
	Subtotal int    `json:"subtotal"`
}
type Order struct {
	Items         []OrderItem `json:"items"`
	FoodSubtotal  int         `json:"food_subtotal"`
	Total         int         `json:"total"`
	DiscountOrFee int         `json:"discount_or_fee"`
}

var nonAlphanumericRegex = regexp.MustCompile(`[^0-9 ]+`)

func clearString(str string) string {
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

// InvoiceUpload ...
func InvoiceUpload(w http.ResponseWriter, r *http.Request) {

	render := marmoset.Render(w, true)

	// Get uploaded file
	r.ParseMultipartForm(32 << 20)
	// upload, h, err := r.FormFile("file")
	upload, _, err := r.FormFile("file")
	if err != nil {
		render.JSON(http.StatusBadRequest, err)
		return
	}
	defer upload.Close()

	// Create physical file
	tempfile, err := ioutil.TempFile("", "ocrserver"+"-")
	if err != nil {
		render.JSON(http.StatusBadRequest, err)
		return
	}
	defer func() {
		tempfile.Close()
		os.Remove(tempfile.Name())
	}()

	// Make uploaded physical
	if _, err = io.Copy(tempfile, upload); err != nil {
		render.JSON(http.StatusInternalServerError, err)
		return
	}

	client := gosseract.NewClient()
	defer client.Close()

	client.SetImage(tempfile.Name())
	client.Languages = []string{"eng"}
	if langs := r.FormValue("languages"); langs != "" {
		client.Languages = strings.Split(langs, ",")
	}
	if whitelist := r.FormValue("whitelist"); whitelist != "" {
		client.SetWhitelist(whitelist)
	}

	var out string
	// switch r.FormValue("format") {
	// case "hocr":
	// out, err = client.HOCRText()

	// default:
	out, err = client.Text()
	// }
	if err != nil {
		render.JSON(http.StatusBadRequest, err)
		return
	}

	//begin grab parser
	orderDetail := Order{}
	// subTotalPriceInt := 0
	orderSummaryFound := false //bool
	subTotalFound := false     //bool
	totalFound := false

	items := []OrderItem{}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		//find order summary
		if strings.Contains(strings.ToLower(line), "order summary") {
			log.Print("order summary found!")
			orderSummaryFound = true
		}

		if orderSummaryFound && len(line) > 2 {

			linerune := []rune(line)
			if linerune[1] == 'x' && unicode.IsNumber(linerune[len(line)-1]) {
				log.Print("item found!")

				//character kedua adalah "x" & character terakhir adalah angka
				//maka dia adalah item
				dats := strings.Split(line, " ")

				//last element is price
				price := strings.TrimSpace(strings.ReplaceAll(dats[len(dats)-1], ".", ""))

				//first element is quantity
				qty := strings.TrimSpace(strings.ReplaceAll(dats[0], "x", ""))

				name := strings.Join(dats[1:len(dats)-1], " ")
				priceInt, _ := strconv.Atoi(price)
				qtyInt, _ := strconv.Atoi(qty)

				items = append(items, OrderItem{
					Name:     name,
					Subtotal: priceInt,
					Qty:      qtyInt,
				})
			}

			if subTotalFound && strings.Contains(strings.ToLower(line), "total") && strings.Contains(strings.ToLower(line), "rp") && !totalFound {
				log.Println("total price found")
				totalFound = true
				dats := strings.Split(line, " ")
				totalPrice := clearString(dats[len(dats)-1])
				totalPriceInt, _ := strconv.Atoi(totalPrice)
				orderDetail.Total = totalPriceInt
				log.Println(totalPriceInt)
			}

			//subtotal and total urutan matters
			if strings.Contains(strings.ToLower(line), "subtotal") && strings.Contains(strings.ToLower(line), "rp") && !subTotalFound {
				log.Println("subtotal found")
				subTotalFound = true
				dats := strings.Split(line, " ")
				subtotalPrice := clearString(dats[len(dats)-1])
				subTotalPriceInt, _ := strconv.Atoi(subtotalPrice)
				orderDetail.FoodSubtotal = subTotalPriceInt
				log.Println(subTotalPriceInt)
			}
		}

	}
	orderDetail.Items = items
	orderDetail.DiscountOrFee = orderDetail.Total - orderDetail.FoodSubtotal

	render.EscapeHTML = false
	// resp, _ := json.Marshal(orderDetail)
	render.JSON(http.StatusOK, orderDetail)
}
