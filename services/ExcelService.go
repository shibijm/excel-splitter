package services

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/xuri/excelize/v2"
)

type ExcelService struct {
	file            *excelize.File
	statusCallbacks map[string]func(string)
}

func NewExcelService() *ExcelService {
	return &ExcelService{
		statusCallbacks: map[string]func(string){},
	}
}

func (excelService *ExcelService) RegisterStatusCallback(id string, statusCallback func(string)) {
	excelService.statusCallbacks[id] = statusCallback
}

func (excelService *ExcelService) dispatchStatus(status string) {
	for _, statusCallback := range excelService.statusCallbacks {
		statusCallback(status)
	}
}

func (excelService *ExcelService) LoadFile(fileReader io.ReadCloser) error {
	defer fileReader.Close()
	file, err := excelize.OpenReader(fileReader)
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}
	excelService.file = file
	return nil
}

func (excelService *ExcelService) DisposeFileIfLoaded() error {
	if excelService.file == nil {
		return nil
	}
	err := excelService.file.Close()
	if err != nil {
		return err
	}
	excelService.file = nil
	return nil
}

func (excelService *ExcelService) GetSheets() []string {
	return excelService.file.GetSheetList()
}

func (excelService *ExcelService) GetColumns(sheet string) ([]string, error) {
	rows, err := excelService.file.Rows(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to open row stream: %w", err)
	}
	hasNext := rows.Next()
	if !hasNext {
		return nil, errors.New("sheet has no data")
	}
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	if !rows.Next() {
		return nil, errors.New("sheet has no data")
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close row stream: %w", err)
	}
	return columns, nil
}

func (excelService *ExcelService) SplitByColumn(sheet string, splitColumnIndex int) error {
	excelService.dispatchStatus("Reading rows")
	rows, err := excelService.file.Rows(sheet)
	if err != nil {
		return fmt.Errorf("failed to get rows iterator: %w", err)
	}
	re, err := regexp.Compile(`[^\w]`)
	if err != nil {
		return fmt.Errorf("failed to compile regex: %w", err)
	}
	var headerRow []string
	var splitColumn string
	cellTypes := map[int]excelize.CellType{}
	cellStyles := map[string]int{}
	columnWidths := map[string]float64{}
	newRows := map[string][][]interface{}{}
	rowIndex := -1
	for rows.Next() {
		rowIndex++
		row, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return fmt.Errorf("failed to read row %d: %w", rowIndex+1, err)
		}
		if rowIndex == 0 {
			headerRow = row
			splitColumn = re.ReplaceAllString(headerRow[splitColumnIndex], " ")
			continue
		}
		if rowIndex == 1 {
			for columnIndex := range row {
				columnName, _ := excelize.ColumnNumberToName(columnIndex + 1)
				axis := fmt.Sprintf("%s%d", columnName, 2)
				cellType, err := excelService.file.GetCellType(sheet, axis)
				if err != nil {
					return fmt.Errorf("failed to get type of cell %s: %w", axis, err)
				}
				cellTypes[columnIndex] = cellType
				cellStyle, err := excelService.file.GetCellStyle(sheet, axis)
				if err != nil {
					return fmt.Errorf("failed to get style of cell %s: %w", axis, err)
				}
				cellStyles[columnName] = cellStyle
				columnWidth, err := excelService.file.GetColWidth(sheet, columnName)
				if err != nil {
					return fmt.Errorf("failed to get width of column %s: %w", columnName, err)
				}
				columnWidths[columnName] = columnWidth
			}
		}
		excelService.dispatchStatus(fmt.Sprintf("Processing row %d", rowIndex))
		var value string
		if len(row) > splitColumnIndex {
			value = re.ReplaceAllString(row[splitColumnIndex], " ")
		}
		if value == "" {
			value = "Blank"
		}
		newRow := []interface{}{}
		for columnIndex, cell := range row {
			cellType := cellTypes[columnIndex]
			var newCell interface{}
			if cell == "" {
				newCell = nil
			} else if cellType == excelize.CellTypeNumber || cellType == excelize.CellTypeUnset {
				cellFloat, err := strconv.ParseFloat(cell, 64)
				if err != nil {
					newCell = cell
				} else {
					newCell = cellFloat
				}
			} else if cellType == excelize.CellTypeBool {
				if cell == "1" {
					newCell = true
				} else if cell == "0" {
					newCell = false
				} else {
					newCell = cell
				}
			} else {
				newCell = cell
			}
			newRow = append(newRow, newCell)
		}
		if v, ok := newRows[value]; ok {
			newRows[value] = append(v, newRow)
		} else {
			newRows[value] = [][]interface{}{newRow}
		}
	}
	if rowIndex < 1 {
		return errors.New("sheet has no data")
	}
	for value, rows := range newRows {
		excelService.dispatchStatus(fmt.Sprintf(`Writing sheet for value "%s"`, value))
		file := excelize.NewFile()
		defer file.Close()
		outputSheet := fmt.Sprintf("%s-%s", splitColumn, value)
		if len(outputSheet) > 31 {
			outputSheet = outputSheet[:31]
		}
		file.SetSheetName("Sheet1", outputSheet)
		err = file.SetSheetRow(outputSheet, "A1", &headerRow)
		if err != nil {
			return fmt.Errorf("failed to set row 1: %w", err)
		}
		for rowIndex, row := range rows {
			axis, _ := excelize.CoordinatesToCellName(1, rowIndex+2)
			err = file.SetSheetRow(outputSheet, axis, &row)
			if err != nil {
				return fmt.Errorf("failed to set row %d: %w", rowIndex+2, err)
			}
		}
		file.Styles = excelService.file.Styles
		for columnName, cellStyle := range cellStyles {
			err := file.SetColStyle(outputSheet, columnName, cellStyle)
			if err != nil {
				return fmt.Errorf("failed to set style of column %s: %w", columnName, err)
			}
		}
		for columnName, columnWidth := range columnWidths {
			err := file.SetColWidth(outputSheet, columnName, columnName, columnWidth)
			if err != nil {
				return fmt.Errorf("failed to set width of column %s: %w", columnName, err)
			}
		}
		headerStyle, err := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Alignment: &excelize.Alignment{Horizontal: "center"}})
		if err != nil {
			return fmt.Errorf("failed to create header style: %w", err)
		}
		err = file.SetRowStyle(outputSheet, 1, 1, headerStyle)
		if err != nil {
			return fmt.Errorf("failed to set style of row 1: %w", err)
		}
		lastColumnName, _ := excelize.ColumnNumberToName(len(headerRow))
		file.AutoFilter(outputSheet, "A1", fmt.Sprintf("%s%d", lastColumnName, len(rows)+1), "")
		file.SetPanes(outputSheet, `{
			"freeze": true,
			"split": false,
			"x_split": 0,
			"y_split": 1,
			"top_left_cell": "A2",
			"active_pane": "bottomLeft",
			"panes": [
				{
					"sqref": "A2",
					"active_cell": "A2",
					"pane": "bottomLeft"
				}
			]
		}`)
		err = os.MkdirAll(splitColumn, 0777)
		if err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		filePath := filepath.Join(splitColumn, fmt.Sprintf("%s-%s-%s.xlsx", sheet, splitColumn, value))
		err = file.SaveAs(filePath)
		if err != nil {
			return fmt.Errorf(`failed to save file for value "%s": %w`, value, err)
		}
		err = file.Close()
		if err != nil {
			return fmt.Errorf(`failed to close file for value "%s": %w`, value, err)
		}
	}
	return nil
}
