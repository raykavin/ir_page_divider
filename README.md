# PDF income statement page divider

## Description

This application allows you to split PDF files of income reports, automatically organizing the extracted documents into folders named according to the company and saving each file with the name of the respective employee. The process includes extracting images from the pages, recognizing text via OCR and segmenting based on the extracted information.

## Features

- Automated processing of PDF files.
- Uses OCR (Optical Character Recognition) to extract information from pages.
- Identifies and organizes files by company and employee.
- Parallel processing for greater efficiency.

## Requirements

To run this project, make sure you have installed:

- [Go](https://golang.org/doc/install) (version 1.18 or higher)
- [Tesseract OCR](https://github.com/tesseract-ocr/tesseract)

## Installation

1. Clone this repository:

   ```sh
   git clone https://github.com/seu-usuario/divisor-pdf.git
   cd divisor-pdf
   ```

2. Install the dependencies:

   ```sh
   go mod tidy
   ```

## Usage

Run the program with the following parameters:

```sh
go run main.go -p <caminho_dos_pdfs> -w <numero_de_workers>
```

### Parameters:
- `-p`: Directory where the PDF files are located (default: current directory `.`).
- `-w`: Number of workers in parallel for processing (default: 5).

Example execution:
```sh
go run main.go -p ./documents -w 10
```

## Project Structure

```
/
├── main.go          # Main application file
├── go.mod           # Project dependencies
├── tmp/             # Temporary directory for image processing
├── processed/       # Output of files organized by company
```

## Main Dependencies

- [go-fitz](https://github.com/karmdip-mi/go-fitz) - PDF image extraction
- [gosseract](https://github.com/otiai10/gosseract) - Wrapper for Tesseract OCR
- [pdfcpu](https://github.com/pdfcpu/pdfcpu) - PDF file manipulation
- [wow](https://github.com/gernest/wow) - Terminal animations

## Contribution

Feel free to contribute improvements to this project. To do so:

1. Fork the repository
2. Create a branch for your feature: `git checkout -b my-feature`
3. Commit your changes: `git commit -m 'Adds new feature'`
4. Push to the branch: `git push origin my-feature`
5. Submit a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for more details.
