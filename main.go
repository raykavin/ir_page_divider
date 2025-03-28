package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gernest/wow"
	"github.com/gernest/wow/spin"
	"github.com/google/goterm/term"
	"github.com/karmdip-mi/go-fitz"
	"github.com/otiai10/gosseract/v2"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// CollectPageInfo groups page information extracted from the PDF.
type CollectPageInfo struct {
	Pages   []string // Collected pages.
	File    string   // PDF file path.
	Key     string   // Unique key (e.g. collaborator) to identify the group.
	SubPath string   // Subdirectory for processed content (e.g. company name)
}

const (
	pdfExt               = ".pdf"
	pngExt               = ".png"
	tmpPath              = "./tmp"
	processedContentPath = "./processed"
	findContentRe        = `(\d{2}\.\d{3}\.\d{3}\/\d{4}-\d{2}|\d{3}.\d{3}.\d{3}-\d{2})\s*(.+)`
	language             = "eng"
)

var (
	rootDir    = flag.String("d", ".", "The PDF files path")
	maxWorkers = flag.Int("w", 5, "The maximum number of workers to process PDFs")
	w          = wow.New(os.Stdout, spin.Get(spin.Dots), "")
	mu         sync.Mutex
)

func main() {
	flag.Parse()
	clear()

	var wg sync.WaitGroup
	workerPool := make(chan struct{}, *maxWorkers)

	// Configure o Tesseract OCR.
	tesseractClient := configureTesseract()

	// Make directories.
	makeDirectories()

	// Channel to collect information from pages.
	collectPageInfoC := make(chan *CollectPageInfo, 1)

	// Channel to signal the end of the collection.
	done := make(chan struct{})

	// Search PDF files in "rootDir" flag
	pdfFiles := searchPdfFiles()
	if len(pdfFiles) == 0 {
		fmt.Printf("No PDF files found on path: %s\n", *rootDir)
	}

	// Goroutine that processes the collected groups.
	go func() {
		for collectedPageInfo := range collectPageInfoC {
			collectPdfPages(collectedPageInfo)
		}

		// After "collectPageInfoC" is closed and the synchronous
		// operation of the function "collectPdfPages" is no longer running
		done <- struct{}{}
	}()

	// Process each PDF in a goroutine.
	for _, pdfFile := range pdfFiles {
		wg.Add(1)
		workerPool <- struct{}{}
		go pdfProcess(pdfFile, tesseractClient, collectPageInfoC, workerPool, &wg)
	}

	wg.Wait()
	close(collectPageInfoC)
	<-done

	tesseractClient.Close()
	os.RemoveAll(tmpPath)

	fmt.Printf("\n\n:) Todos os arquivos foram processados com sucesso")
}

func configureTesseract() *gosseract.Client {
	tesseractClient := gosseract.NewClient()
	tesseractClient.SetLanguage(language)
	return tesseractClient
}

func makeDirectories() {
	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		os.MkdirAll(tmpPath, os.ModePerm)
	}
	if _, err := os.Stat(processedContentPath); os.IsNotExist(err) {
		os.MkdirAll(processedContentPath, os.ModePerm)
	}
}

func searchPdfFiles() []string {
	var pdfFiles []string
	err := filepath.Walk(*rootDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error acessing file %s: %v", path, err)
			return nil
		}
		if filepath.Ext(path) == pdfExt {
			pdfFiles = append(pdfFiles, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error searching PDF files: %v", err)
	}
	return pdfFiles
}

func pdfProcess(file string, tesseractClient *gosseract.Client, collectPageInfoC chan<- *CollectPageInfo, workerPoolC chan struct{}, wg *sync.WaitGroup) {
	defer func() {
		<-workerPoolC
		wg.Done()
	}()

	// Initialize the current group with the file.
	currentGroup := &CollectPageInfo{File: file}
	pdf, err := fitz.New(file)
	if err != nil {
		log.Printf("Error opening PDF file %s: %v", file, err)
		return
	}
	defer pdf.Close()

	pdfTotalPages := pdf.NumPage()

	for pageNumber := 0; pageNumber < pdfTotalPages; pageNumber++ {
		// Extract the image from the page.
		img, err := pdf.Image(pageNumber)
		if err != nil {
			log.Printf("Error reading page %d of file %s: %v", pageNumber+1, file, err)
			continue
		}

		path, err := saveImageAsPng(img, file, pageNumber)
		if err != nil {
			continue
		}

		content, err := ocrReadFromFile(path, tesseractClient)
		if err != nil || content == nil {
			log.Printf("Invalid content or error reading OCR file %s", path)
			continue
		}

		// Extract the information using regex.
		contentPattern := findContent(*content)

		company := extractField(contentPattern, 2, 0)
		collaborator := extractField(contentPattern, 2, 1)

		// If no collaborator was found on the page, use the previous collaborator (if any).
		if collaborator == "" && currentGroup.Key != "" {
			collaborator = currentGroup.Key
		}

		// If the key is already set and the current one is different, send the current group and restart the accumulation.
		if currentGroup.Key != "" && collaborator != currentGroup.Key {
			collectPageInfoC <- currentGroup
			currentGroup = &CollectPageInfo{File: file}
		}

		if company != "" {
			currentGroup.SubPath = company
		}

		// Update the key only if the collaborator is defined.
		if collaborator != "" {
			currentGroup.Key = collaborator
		}

		currentGroup.Pages = append(currentGroup.Pages, fmt.Sprint(pageNumber+1))

		w.Text(term.Bluef(" Please wait, processing file: %s... %d/%d page(s)", file, pageNumber+1, pdfTotalPages))
		w.Start()
	}

	// Send the last group, ensuring that the last page is processed.
	collectPageInfoC <- currentGroup
}

func saveImageAsPng(img image.Image, file string, pageNumber int) (string, error) {
	filename := fmt.Sprintf("%s_%d%s", strings.Split(filepath.Base(file), ".")[0], pageNumber+1, pngExt)
	path := filepath.Join(tmpPath, filename)

	fStream, err := os.Create(path)
	if err != nil {
		log.Printf("Error creating temporary PNG file %s: %v", path, err)
		return "", err
	}
	defer fStream.Close()

	if err := png.Encode(fStream, img); err != nil {
		log.Printf("Error encoding page %d to PNG in file %s: %v", pageNumber+1, file, err)
		return "", err
	}
	return path, nil
}

func extractField(contentPattern []string, length, index int) string {
	if index < len(contentPattern) {
		parts := strings.SplitN(contentPattern[index], " ", 2)
		if len(parts) >= length {
			return parts[1]
		}
		return parts[0]
	}
	return ""
}

func collectPdfPages(collectPageInfo *CollectPageInfo) {
	pageOffset := collectPageInfo.Pages
	inFile := collectPageInfo.File
	fileName := collectPageInfo.Key
	outFile := filepath.Join(processedContentPath, collectPageInfo.SubPath, fmt.Sprintf("%s%s", fileName, pdfExt))

	if _, err := os.Stat(filepath.Dir(outFile)); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(outFile), os.ModePerm)
	}

	if err := api.CollectFile(inFile, outFile, pageOffset, api.LoadConfiguration()); err != nil {
		fmt.Printf("Error processing file: %s for: %s", inFile, fileName)
	}
}

func ocrReadFromFile(path string, client *gosseract.Client) (*string, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := client.SetImage(path); err != nil {
		return nil, err
	}

	content, err := client.Text()
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func findContent(content string) []string {
	re := regexp.MustCompile(findContentRe)
	return re.FindAllString(content, -1)
}

func clear() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}
