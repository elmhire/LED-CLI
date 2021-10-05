package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
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
	fmt.Printf("Opening %s...\n\n", filename)
	data := getRawFileDataAsStr(filename)
	content := convertToUTF8(data)

	// Extract links from contents
	links, err := extractLinks(content)
	if err != nil {
		fmt.Println("No files found to download.")
		normalExit()
	}

	numLinks := len(links)

	// Download links
	fmt.Printf("%d file's to download...\n", numLinks)
	complete := downloadLinks(links, numLinks)
	fmt.Printf("%d of %d files downloaded completely.\n\n", complete, numLinks)

	fmt.Println("Renaming files...")
	renameFiles(numLinks)
	fmt.Printf("%d of %d files renamed successfully.\n\n", complete, numLinks)
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
		fmt.Println("\nInvalid entry, please try again.")
		fmt.Println()
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

func getEMLContent(filename string) string {
	email := getEmailContent(filename)
	html := email.HTMLBody
	decoded, err := base64.StdEncoding.DecodeString(html)
	if err != nil {
		fmt.Println("Data not base64 encoded.")
		fmt.Println()
		return html
	}
	fmt.Println("Data is base64 encoded.")
	fmt.Println()
	return string(decoded)
}

func getEmailContent(filename string) parsemail.Email {
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
	return email
}

func getHTMLContent(filename string) string {
	temp, err := os.ReadFile(filename)
	check(err)
	return string(temp)
}

func getBytesReaderFromFile(filename string) *bytes.Reader {
	dat, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("Failed opening %s: %s", filename, err)
		pause()
		os.Exit(1)
	}
	return bytes.NewReader(dat)
}

func convertToUTF8(data string) string {
	content, err := gonvert.New(data, gonvert.UTF8).Convert()
	if err != nil {
		fmt.Println("Failed to Convert: ", err)
		pause()
		os.Exit(1)
	}
	return content
}

// Link ...
type Link struct {
	url  string
	text string
}

func extractLinks(htmlText string) ([]Link, error) {
	var links []Link

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
		return nil, errors.New("no links found")
	}
	return links, nil
}

func downloadLinks(links []Link, total int) int {
	complete := 0
	for _, link := range links {
		fmt.Printf("Downloading %s %d of %d...", link.text, (complete + 1), total)
		err := downloadFile(link, true)
		if err != nil {
			fmt.Printf("\terror, incomplete!\n")
			continue
		}
		fmt.Printf("\tcomplete.\n")
		complete++
	}
	return complete
}

func downloadFile(link Link, useLinkName bool, pathOptional ...string) error {
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

	return err
}

func renameFiles(total int) {
	pdf.DebugOn = true

	files := getPdfFiles()
	for i, file := range files {
		invoice, err := readPdf(file) // Read local pdf file
		if err != nil {
			panic(err)
		}
		billToName := getShipToName(invoice)
		src, dest := file, strings.Join([]string{billToName, file}, "_")
		fmt.Printf("Renaming %d of %d: %s to %s\n", (i + 1), total, src, dest)
		os.Rename(src, dest)
	}
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

func readPdf(path string) (string, error) {
	f, r, err := pdf.Open(path)
	// remember close file
	defer f.Close()
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	buf.ReadFrom(b)
	return buf.String(), nil
}

func getShipToName(pdfStr string) string {
	var re = regexp.MustCompile(`('|,)`)
	var inParens = false
	pdfStr = pdfStr[strings.Index(pdfStr, "SHIP TO:")+8:]

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
			pdfStr = strings.Replace(pdfStr[:i], " ", "_", -1)
			break
		}
	}
	pdfStr = re.ReplaceAllString(pdfStr, ``)
	return pdfStr
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
