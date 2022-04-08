package ledcli

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

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

func GetFileName() (choice string) {
	files := GetCwdEmailList()
	for {
		fmt.Printf("Please select the email to open.\n\n")

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

// getRawFileDataAsStr
func GetRawFileDataAsStr(filename string) (str string) {
	if strings.Contains(filename, "eml") {
		str = string(getEMLContent(filename))
	} else {
		str = string(getHTMLContent(filename))
	}
	return
}

// getEMLContent
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

// getEmailContent
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

// getHTMLContent
func getHTMLContent(fn string) (c string) {
	temp, err := os.ReadFile(fn)
	check(err)

	c = string(temp)
	return
}

// getBytesReaderFromFile
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

// convertToUTF8
func ConvertToUTF8(dat string) (s string) {
	s, err := gonvert.New(dat, gonvert.UTF8).Convert()
	if err != nil {
		fmt.Println("Failed to Convert: ", err)
		pause()
		os.Exit(1)
	}
	return
}

// extractLinks
func ExtractLinks(htmlText string) (links []Link, e error) {
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
					links = append(links, *newLink(a.Val, n.FirstChild.Data))
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

func DownloadLinks(links []Link) (complete int) {
	for i, link := range links {
		fmt.Printf("Downloading %s %d of %d...", link.text, i+1, len(links))
		if err := link.download(true); err != nil {
			fmt.Printf("\terror, incomplete!\n")
			continue
		}
		fmt.Printf("\tcomplete.\n")
		complete++
	}
	return
}

func GetDataFromFiles() (csvEntries []InvoiceEntry) {
	pdf.DebugOn = true
	var pdfList []string

	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if fName := file.Name(); strings.HasPrefix(fName, "SI-") &&
			strings.HasSuffix(fName, ".pdf") {
			pdfList = append(pdfList, fName)
		}
	}

	for _, pdfName := range pdfList {
		csvEntries = append(csvEntries, *newEntry(pdfName))
	}
	return
}

func escapeString(str string) string {
	// var re = regexp.MustCompile(`('|,)`)
	return regexp.MustCompile(`('|,)`).ReplaceAllString(
		strings.Replace(str, " ", "_", -1),
		``,
	)
}

func RenameFile(index int, total int, entry InvoiceEntry) {
	origName, newName := entry.GetFileNames()
	os.Rename(origName, newName)
	fmt.Printf(
		"Renaming %d of %d: %s to %s\n",
		(index + 1),
		total,
		origName,
		newName,
	)
}

func NormalExit() {
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
