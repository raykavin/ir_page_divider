# Divisor de Páginas de Arquivos PDF

## Descrição

Este aplicativo permite dividir arquivos PDF de informes de rendimentos, organizando automaticamente os documentos extraídos em pastas nomeadas conforme a empresa e salvando cada arquivo com o nome do respectivo colaborador. O processo inclui a extração de imagens das páginas, reconhecimento de texto via OCR e a segmentação baseada nas informações extraídas.

## Funcionalidades

- Processamento automatizado de arquivos PDF.
- Utiliza OCR (Reconhecimento Óptico de Caracteres) para extrair informações das páginas.
- Identifica e organiza arquivos por empresa e colaborador.
- Processamento paralelo para maior eficiência.

## Requisitos

Para executar este projeto, certifique-se de ter instalado:

- [Go](https://golang.org/doc/install) (versão 1.18 ou superior)
- [Tesseract OCR](https://github.com/tesseract-ocr/tesseract)

## Instalação

1. Clone este repositório:

   ```sh
   git clone https://github.com/seu-usuario/divisor-pdf.git
   cd divisor-pdf
   ```

2. Instale as dependências:

   ```sh
   go mod tidy
   ```

## Uso

Execute o programa com os seguintes parâmetros:

```sh
go run main.go -p <caminho_dos_pdfs> -w <numero_de_workers>
```

### Parâmetros:
- `-p`: Diretório onde estão localizados os arquivos PDF (padrão: diretório atual `.`).
- `-w`: Número de workers em paralelo para processamento (padrão: 5).

Exemplo de execução:
```sh
go run main.go -p ./documentos -w 10
```

## Estrutura do Projeto

```
/
├── main.go          # Arquivo principal do aplicativo
├── go.mod           # Dependências do projeto
├── tmp/             # Diretório temporário para processamento de imagens
├── processed/       # Saída dos arquivos organizados por empresa e colaborador
```

## Dependências Principais

- [go-fitz](https://github.com/karmdip-mi/go-fitz) - Extração de imagens de PDFs
- [gosseract](https://github.com/otiai10/gosseract) - Wrapper para Tesseract OCR
- [pdfcpu](https://github.com/pdfcpu/pdfcpu) - Manipulação de arquivos PDF
- [wow](https://github.com/gernest/wow) - Animações para terminal

## Contribuição

Sinta-se à vontade para contribuir com melhorias para este projeto. Para isso:

1. Fork o repositório
2. Crie uma branch para sua feature: `git checkout -b minha-feature`
3. Commit suas mudanças: `git commit -m 'Adiciona nova funcionalidade'`
4. Push para a branch: `git push origin minha-feature`
5. Envie um Pull Request

## Licença

Este projeto está licenciado sob a MIT License - veja o arquivo [LICENSE](LICENSE) para mais detalhes.

