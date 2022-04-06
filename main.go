package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"

	ledcli "github.com/elmhire/LED-CLI/src"
)

func main() {
	// Get email file name input from user
	filename := ledcli.GetFileName()

	// Get data from file
	fmt.Printf("Opening %s...", filename)
	data := ledcli.GetRawFileDataAsStr(filename)
	content := ledcli.ConvertToUTF8(data)
	fmt.Printf("Success!\n\n")

	// Extract links from contents
	links, err := ledcli.ExtractLinks(content)
	if err != nil {
		fmt.Println("No files found to download. Exiting.")
		ledcli.NormalExit()
	}

	// Download links
	fmt.Printf("%d file's to download...\n", len(links))
	complete := ledcli.DownloadLinks(links)
	fmt.Printf("%d of %d files downloaded completely.\n\n", complete, len(links))

	// Gather data for future CSV output
	fmt.Printf("Parsing downloaded data... ")
	csvData := ledcli.GetDataFromFiles()

	csvText := [][]string{
		{"location", "invoice_number", "total"},
	}

	for _, e := range csvData {
		csvText = append(csvText, []string{e.GetLocation(), e.GetInvoiceNum(), e.GetTotal()})
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
		ledcli.RenameFile(i, len(csvData), invoice)
	}
	fmt.Printf("%d of %d files renamed successfully.\n\n", complete, len(links))

	ledcli.NormalExit()
} // End main
