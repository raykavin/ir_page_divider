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

// CollectPageInfo agrupa as informações de páginas extraídas do PDF.
type CollectPageInfo struct {
	Pages   []string // Páginas coletadas.
	File    string   // Caminho do arquivo PDF.
	Key     string   // Chave única (ex: colaborador) para identificar o grupo.
	SubPath string   // Subdiretório para o conteúdo processado (ex: nome da empresa)
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
	rootDir    = flag.String("p", ".", "O caminho dos arquivos PDF")
	maxWorkers = flag.Int("w", 5, "O número máximo de workers para processar os PDFs")
	w          = wow.New(os.Stdout, spin.Get(spin.Dots), "")
	mu         sync.Mutex
)

func main() {
	flag.Parse()
	clear()

	var wg sync.WaitGroup
	workerPool := make(chan struct{}, *maxWorkers)

	// Configura o Tesseract OCR.
	tesseractClient := configureTesseract()

	// Cria diretórios necessários.
	makeDirectories()

	// Canal para coletar informações das páginas.
	collectPageInfoChannel := make(chan *CollectPageInfo, 1)

	// Canal para sinalizar o término da coleta.
	done := make(chan struct{})

	// Busca arquivos PDF.
	pdfFiles := searchPdfFiles()
	if len(pdfFiles) == 0 {
		fmt.Printf("Nenhum arquivo PDF encontrado no caminho: %s\n", *rootDir)
	}

	// Goroutine que processa os grupos coletados.
	go func() {
		for collectedPageInfo := range collectPageInfoChannel {
			collectPdfPages(collectedPageInfo)
		}
		done <- struct{}{}
	}()

	// Processa cada PDF em uma goroutine.
	for _, pdfFile := range pdfFiles {
		wg.Add(1)
		workerPool <- struct{}{}
		go pdfProcess(pdfFile, tesseractClient, collectPageInfoChannel, workerPool, &wg)
	}

	wg.Wait()
	close(collectPageInfoChannel)
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
			log.Printf("Erro ao acessar o arquivo %s: %v", path, err)
			return nil
		}
		if filepath.Ext(path) == pdfExt {
			pdfFiles = append(pdfFiles, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Erro ao buscar arquivos PDF: %v", err)
	}
	return pdfFiles
}

func pdfProcess(file string, tesseractClient *gosseract.Client, collectPageInfoChannel chan<- *CollectPageInfo, workerPoolSignal chan struct{}, wg *sync.WaitGroup) {
	defer func() {
		<-workerPoolSignal
		wg.Done()
	}()

	// Inicializa o grupo atual com o arquivo.
	currentGroup := &CollectPageInfo{File: file}
	pdf, err := fitz.New(file)
	if err != nil {
		log.Printf("Erro ao abrir o arquivo PDF %s: %v", file, err)
		return
	}
	defer pdf.Close()

	pdfTotalPages := pdf.NumPage()

	for pageNumber := 0; pageNumber < pdfTotalPages; pageNumber++ {
		// Extrai a imagem da página.
		img, err := pdf.Image(pageNumber)
		if err != nil {
			log.Printf("Erro ao ler a página %d do arquivo %s: %v", pageNumber+1, file, err)
			continue
		}

		path, err := saveImageAsPng(img, file, pageNumber)
		if err != nil {
			continue
		}

		content, err := ocrReadFromFile(path, tesseractClient)
		if err != nil || content == nil {
			log.Printf("Conteúdo inválido ou erro na leitura OCR do arquivo %s", path)
			continue
		}

		// Extrai as informações usando regex.
		contentPattern := findContent(*content)
		companyName, collaborator := extractCompanyAndCollaborator(contentPattern)

		// Se não encontrou colaborador na página, usa o colaborador anterior (se existir).
		if collaborator == "" && currentGroup.Key != "" {
			collaborator = currentGroup.Key
		}

		// Se a chave já estiver definida e a atual for diferente, envia o grupo atual e reinicia a acumulação.
		if currentGroup.Key != "" && collaborator != currentGroup.Key {
			collectPageInfoChannel <- currentGroup
			currentGroup = &CollectPageInfo{File: file}
		}

		if companyName != "" {
			currentGroup.SubPath = companyName
		}
		// Atualiza a chave somente se o colaborador estiver definido.
		if collaborator != "" {
			currentGroup.Key = collaborator
		}

		currentGroup.Pages = append(currentGroup.Pages, fmt.Sprint(pageNumber+1))

		w.Text(term.Bluef("Aguarde, processando o arquivo: %s... %d/%d página(s)", file, pageNumber+1, pdfTotalPages))
		w.Start()
	}

	// Envia o último grupo, garantindo que a última página seja processada.
	collectPageInfoChannel <- currentGroup
}

func saveImageAsPng(img image.Image, file string, pageNumber int) (string, error) {
	filename := fmt.Sprintf("%s_%d%s", strings.Split(filepath.Base(file), ".")[0], pageNumber+1, pngExt)
	path := filepath.Join(tmpPath, filename)

	fStream, err := os.Create(path)
	if err != nil {
		log.Printf("Erro ao criar o arquivo PNG temporário %s: %v", path, err)
		return "", err
	}
	defer fStream.Close()

	if err := png.Encode(fStream, img); err != nil {
		log.Printf("Erro ao codificar a página %d para PNG no arquivo %s: %v", pageNumber+1, file, err)
		return "", err
	}
	return path, nil
}

func extractCompanyAndCollaborator(contentPattern []string) (string, string) {
	companyName := ""
	collaborator := ""

	if len(contentPattern) >= 2 {
		companyParts := strings.SplitN(contentPattern[0], " ", 2)
		if len(companyParts) >= 2 {
			companyName = companyParts[1]
		} else {
			companyName = strings.Split(contentPattern[0], " ")[0]
		}

		employeeParts := strings.SplitN(contentPattern[1], " ", 2)
		if len(employeeParts) >= 2 {
			collaborator = employeeParts[1]
		} else {
			collaborator = strings.Split(contentPattern[1], " ")[0]
		}
	}

	return companyName, collaborator
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
		fmt.Printf("Erro ao processar o arquivo: %s para: %s", inFile, fileName)
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
