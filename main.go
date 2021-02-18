package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"
)

var apiKey string

const postUrl = "https://www.app.fattura24.com/api/v0.3/SaveDocument"

func main() {
	flag.StringVar(&apiKey, "apiKey", "", "API key")
	flag.Parse()
	args := flag.Args()
	log.SetFlags(0)

	if len(args) == 0 {
		log.Fatal(`No input files provided.
Usage: fattura24 -apiKey=whatever table.csv`)
	}

	for _, fileName := range args {
		fattPostCsv(fileName)
	}
}

func fattPostCsv(fileName string) {
	f, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fattPostRecord(rec)
	}
}

var tpl *template.Template

func init() {
	var err error
	tpl, err = template.New("tpl").Parse(`<Fattura24>
    <Document>
        <DocumentType>{{.DocType}}</DocumentType>
        <CustomerName>{{.Customer}}</CustomerName>
        <CustomerAddress>{{.Address}}</CustomerAddress>
        <CustomerPostcode>{{.PostCode}}</CustomerPostcode>
        <CustomerCity>{{.City}}</CustomerCity>
        <CustomerCountry>{{.Country}}</CustomerCountry>
        <CustomerFiscalCode>{{.FiscalCode}}</CustomerFiscalCode>
        <CustomerVatCode>{{.VatCode}}</CustomerVatCode>
        <TotalWithoutTax>{{.WithoutTax}}</TotalWithoutTax>
        <VatAmount>{{.Tax}}</VatAmount>
        <Total>{{.Total}}</Total>
        <SendEmail>false</SendEmail>
        <Rows>
            <Row>
                <Description>-</Description>
                <Qty>1</Qty>
                <Price>{{.WithoutTax}}</Price>
				<VatCode>22</VatCode>
                <VatDescription>22%</VatDescription>
            </Row>
        </Rows>
    </Document>
</Fattura24>`)
	if err != nil {
		panic(err)
	}
}

type Line struct {
	DocType, Customer, Address, PostCode, City, Country, FiscalCode, VatCode string
	WithoutTax, Tax, Total                                                   string
}

const iva = 0.22

func fattPostRecord(rec []string) {
	line := Line{rec[0], rec[1], rec[2], rec[3], rec[4], rec[5], rec[6], rec[7], rec[8], "", ""}
	wt, err := strconv.ParseFloat(line.WithoutTax, 64)
	if err != nil {
		log.Fatal(err)
	}
	tax := wt * iva
	tot := wt + tax
	line.Tax = strconv.FormatFloat(tax, 'f', 2, 64)
	line.Total = strconv.FormatFloat(tot, 'f', 2, 64)
	var buf bytes.Buffer
	err = tpl.Execute(&buf, line)
	if err != nil {
		log.Fatal(err)
	}
	xml := buf.String()

	resp, err := http.PostForm(postUrl, url.Values{"apiKey": {apiKey}, "xml": {xml}})
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	// The endpoint seems to return code 200 even in case of errors.
	// In case of success, the response contains "Operation completed" or "Operazione completata"
	s := string(body)
	if !strings.Contains(s, "complet") || strings.Contains(s, "error") {
		log.Fatalf("Unexpected response with code %d:\n%s", resp.StatusCode, body)
	}
}
