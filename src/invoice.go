package ledcli

import (
	"bytes"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

type InvoiceEntry struct {
	originalFileName string
	newFileName      string
	location         string
	invoice_number   string
	total            string
}

func newEntry(fileName string) (i *InvoiceEntry) {
	i = &InvoiceEntry{originalFileName: fileName}

	invoice, err := readPdf(fileName) // Read local pdf file
	if err != nil {
		panic(err)
	}
	i.location = getShipToName(invoice)
	i.invoice_number = strings.TrimRight(fileName, ".pdf")
	i.newFileName = strings.Join([]string{escapeString(i.location), i.originalFileName}, "_")
	i.total = getTotal(invoice)

	return
}

func readPdf(path string) (bs string, err error) {
	f, r, err := pdf.Open(path)
	// remember close file
	defer f.Close()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		bs = ""
		return
	}
	buf.ReadFrom(b)
	bs = buf.String()
	return
}

func getShipToName(pdfStr string) (s string) {
	var inParens = false
	pdfStr = pdfStr[strings.Index(pdfStr, "SHIP TO:")+8:]

	// Loop through pdfStr one character at a time (RUNE)
	// until curChar is a number digit after a close paren
	// that marks the end of the "Ship To" name.
	for i, curChar := range pdfStr {
		if string(curChar) == "(" {
			inParens = true
		}
		if inParens {
			if string(curChar) == ")" {
				inParens = false
			} else {
				continue
			}
		}
		if unicode.IsDigit(curChar) {
			s = strings.TrimSpace(pdfStr[:i])
			break
		}
	}
	return
}

func getTotal(pdfStr string) string {
	var pleaseLoc = strings.Index(pdfStr, "Please")
	return strings.TrimSpace(
		strings.Trim(
			pdfStr[pleaseLoc-8:pleaseLoc], "$"),
	)
}

// Returns original file name and the new file name as strings
func (e InvoiceEntry) GetFileNames() (string, string) {
	return e.originalFileName, e.newFileName
}

func (e InvoiceEntry) GetLocation() string {
	return e.location
}

func (e InvoiceEntry) GetInvoiceNum() string {
	return e.invoice_number
}

func (e InvoiceEntry) GetTotal() string {
	return e.total
}
