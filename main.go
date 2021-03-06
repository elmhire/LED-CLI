package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/DusanKasan/parsemail"
	"github.com/ledongthuc/pdf"
	"github.com/timakin/gonvert"
	"golang.org/x/net/html"
)

func check(err error) {
	if err != nil {
		fmt.Printf("Failed in check(): %s\n", err)
		pause()
		os.Exit(1)
	}
}

func main() {
	// Get email file name input from user
	filename := getFileName()

	// Get data from file
	fmt.Printf("Opening %s...", filename)
	data := getRawFileDataAsStr(filename)
	content := convertToUTF8(data)
	fmt.Printf("Success!\n\n")

	// Extract links from contents
	links, err := extractLinks(content)
	if err != nil {
		fmt.Println("No files found to download. Exiting.")
		normalExit()
	}

	// Download links
	fmt.Printf("%d file's to download...\n", len(links))
	complete := downloadLinks(links)
	fmt.Printf("%d of %d files downloaded completely.\n\n", complete, len(links))

	// Gather data for future CSV output
	fmt.Printf("Parsing downloaded data... ")
	csvData := getDataFromFiles()

	csvText := [][]string{
		{"location", "invoice_number", "total"},
	}

	for _, e := range csvData {
		csvText = append(csvText, []string{e.location, e.invoice_number, e.total})
	}

	fmt.Printf("Success!\n\n")

	// Write csv file
	fmt.Printf("Writing data to 'downloaded_invoice_data.csv'... ")
	f, err := os.Create("downloaded_invoice_data.csv")
	defer f.Close()
	if err != nil {
		log.Fatalln("Failed to create file", err)
	}

	w := csv.NewWriter(f)
	err = w.WriteAll(csvText)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Success!\n\n")

	// Rename files
	fmt.Println("Renaming files...")
	for i, invoice := range csvData {
		renameFile(i, len(csvData), invoice)
	}
	fmt.Printf("%d of %d files renamed successfully.\n\n", complete, len(links))

	normalExit()
} // End main

func getFileName() (choice string) {
	files := GetCwdEmailList()
	for {
		fmt.Println("Please select the email to open.")
		fmt.Println()
		for i, file := range files {
			fmt.Printf("(%d) %s\n", i+1, file)
		}

		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("\n (q to exit)-> ")
		input, _ := reader.ReadString('\n')

		input = strings.TrimSpace(input)
		if strings.ToLower(input) == "q" {
			fmt.Println("Exiting...")
			pause()
			os.Exit(0)
		}
		ans, err := strconv.ParseUint(input, 10, 16)
		// If valid entry
		if err == nil && (int(ans) > 0 && int(ans) <= len(files)) {
			fmt.Printf("\nYou selected: %v \n\n", ans)
			choice = files[ans-1]
			break
		}
		// else
		fmt.Printf("\nInvalid entry, please try again.\n")
	}
	return
}

// GetCwdEmailList returns a []string with the names of each file that could
// be an email file name in the current working directory.
func GetCwdEmailList() (listing []string) {
	var fTypes = []string{"htm", "html", "eml"}

	list, err := ioutil.ReadDir(".")
	check(err)

	for _, f := range list {
		if HasSuffix(fTypes, f.Name()) {
			listing = append(listing, f.Name())
		}
	}
	return
}

// HasSuffix receives a list of strings and another string aStr to be checked.
// If any string in the list is a suffix of aStr, then HasSuffix will return
// true. Otherwise, it will return false.
func HasSuffix(list []string, aStr string) bool {
	for _, b := range list {
		if strings.HasSuffix(aStr, strings.ToLower(b)) {
			return true
		}
	}
	return false
}

func getRawFileDataAsStr(filename string) (str string) {
	if strings.Contains(filename, "eml") {
		str = string(getEMLContent(filename))
	} else {
		str = string(getHTMLContent(filename))
	}
	return
}

func getEMLContent(filename string) (decodedString string) {
	email := getEmailContent(filename)
	html := email.HTMLBody
	decoded, err := base64.StdEncoding.DecodeString(html)
	if err != nil {
		fmt.Printf("Data not base64 encoded.\n\n")
		decodedString = html
		return
	}
	fmt.Printf("Data is base64 encoded.\n\n")
	decodedString = string(decoded)
	return
}

func getEmailContent(filename string) (email parsemail.Email) {
	rd := getBytesReaderFromFile(filename)

	email, err := parsemail.Parse(rd)
	if err == io.EOF {
		fmt.Println("Empty file:", filename, "\nExiting.")
		pause()
		os.Exit(0)
	} else if err != nil {
		fmt.Println("Error reading file:", filename)
		fmt.Println("Either it is corrupt or it is not an 'eml' file.")
		pause()
		os.Exit(1)
	}
	return
}

func getHTMLContent(fn string) (c string) {
	temp, err := os.ReadFile(fn)
	check(err)

	c = string(temp)
	return
}

func getBytesReaderFromFile(fn string) (br *bytes.Reader) {
	dat, err := os.ReadFile(fn)
	if err != nil {
		log.Printf("Failed opening %s: %s", fn, err)
		pause()
		os.Exit(1)
	}
	br = bytes.NewReader(dat)
	return
}

func convertToUTF8(dat string) (s string) {
	s, err := gonvert.New(dat, gonvert.UTF8).Convert()
	if err != nil {
		fmt.Println("Failed to Convert: ", err)
		pause()
		os.Exit(1)
	}
	return
}

// Link ...
type Link struct {
	url  string
	text string
}

func extractLinks(htmlText string) (links []Link, e error) {
	node, err := html.Parse(strings.NewReader(htmlText))
	check(err)

	// Algorithm taken from https://pkg.go.dev/golang.org/x/net/html#pkg-functions
	// Takes an html.Node and walks the tree recursively
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" &&
					!strings.Contains(a.Val, "mailto") &&
					!strings.Contains(n.FirstChild.Data, "here") {
					link := Link{
						url:  a.Val,
						text: n.FirstChild.Data,
					}
					links = append(links, link)
					break
				} // End if
			} // End For
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(node)
	if links == nil {
		e = errors.New("no links found")
		return
	}
	return
}

func downloadLinks(links []Link) (complete int) {
	for i, link := range links {
		fmt.Printf("Downloading %s %d of %d...", link.text, i+1, len(links))
		err := downloadFile(link, true)
		if err != nil {
			fmt.Printf("\terror, incomplete!\n")
			continue
		}
		fmt.Printf("\tcomplete.\n")
		complete++
	}
	return
}

func downloadFile(link Link, useLinkName bool, pathOptional ...string) (err error) {
	var (
		path     string
		filename string
	)

	if len(pathOptional) > 0 {
		path = pathOptional[0]
	} else {
		path = "."
	}

	response, err := http.Get(link.url)
	check(err)

	defer response.Body.Close()

	// Decide whether to use link.text as filename
	// or the name that the server gives us
	// Right now we only use link.text as filename
	if useLinkName {
		filename = fmt.Sprintf("%s/%s", path, link.text)
	} else {
		filename = filepath.Base(response.Request.URL.EscapedPath())
		temp, err := url.PathUnescape(filename)
		if err == nil {
			filename = temp
		}
	}

	// Create the file
	out, err := os.Create(fmt.Sprintf("%s/%s", path, filename))
	if err != nil {
		return err
	}

	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, response.Body)
	out.Close()

	return
}

// CSVEntry_T ...
type CSVEntry_T struct {
	originalFileName string
	newFileName      string
	location         string
	invoice_number   string
	total            string
}

func newEntry(fileName string) (e *CSVEntry_T) {
	e = &CSVEntry_T{originalFileName: fileName}

	invoice, err := readPdf(fileName) // Read local pdf file
	if err != nil {
		panic(err)
	}
	e.location = getShipToName(invoice)
	e.invoice_number = strings.TrimRight(fileName, ".pdf")
	e.newFileName = strings.Join([]string{escapeString(e.location), e.originalFileName}, "_")
	e.total = getTotal(invoice)

	return
}

func getDataFromFiles() (csvEntries []CSVEntry_T) {
	pdf.DebugOn = true
	files := getPdfFiles()

	for _, file := range files {
		csvEntries = append(csvEntries, *newEntry(file))
	}
	return
}

func getPdfFiles() (pdfList []string) {
	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "SI-") && strings.Contains(name, ".pdf") {
			pdfList = append(pdfList, name)
		}
	}
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
	var totalLoc = strings.LastIndex(pdfStr, "TOTAL") + len("TOTAL")
	// var total = totalLoc + 5
	return strings.TrimSpace(
		strings.Trim(
			pdfStr[totalLoc:totalLoc+8], "$"),
	)
}

func escapeString(str string) string {
	// var re = regexp.MustCompile(`('|,)`)
	return regexp.MustCompile(`('|,)`).ReplaceAllString(
		strings.Replace(str, " ", "_", -1),
		``,
	)
}

func renameFile(index int, total int, entry CSVEntry_T) {
	os.Rename(entry.originalFileName, entry.newFileName)
	fmt.Printf(
		"Renaming %d of %d: %s to %s\n",
		(index + 1),
		total,
		entry.originalFileName,
		entry.newFileName,
	)
}

func normalExit() {
	fmt.Print("\nPress 'Enter' to continue...")
	pause()
	os.Exit(0)
}

func pause() {
	_, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		log.Fatal("Something went wrong: ", err)
	}
}
