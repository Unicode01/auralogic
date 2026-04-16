package admin

import (
	"encoding/csv"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

func readAdminTabularRows(file *multipart.FileHeader) (string, [][]string, error) {
	if file == nil {
		return "", nil, fmt.Errorf("missing file")
	}

	extension := strings.ToLower(strings.TrimSpace(filepath.Ext(file.Filename)))
	src, err := file.Open()
	if err != nil {
		return "", nil, err
	}
	defer func() {
		_ = src.Close()
	}()

	switch extension {
	case ".xlsx":
		rows, err := readAdminXLSXRows(src)
		return "xlsx", rows, err
	case ".csv":
		rows, err := readAdminCSVRows(src)
		return "csv", rows, err
	default:
		return "", nil, fmt.Errorf("unsupported format")
	}
}

func readAdminCSVRows(src multipart.File) ([][]string, error) {
	reader := csv.NewReader(src)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	rows := make([][]string, 0)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, record)
	}
	return rows, nil
}

func readAdminXLSXRows(src multipart.File) ([][]string, error) {
	workbook, err := excelize.OpenReader(src)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = workbook.Close()
	}()

	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("empty workbook")
	}

	rows, err := workbook.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	return rows, nil
}
