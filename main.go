package main

import (
	"excel-splitter/services"
	"excel-splitter/ui"
)

func main() {
	excelService := services.NewExcelService()
	mainWindow := ui.NewMainWindow(excelService)
	mainWindow.Run()
}
