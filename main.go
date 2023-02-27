package main

import (
	"excel-splitter/services"
	"excel-splitter/ui"
	"fmt"
	"os"
)

func main() {
	excelService := services.NewExcelService()
	mainWindow := ui.NewMainWindow(excelService)
	err := mainWindow.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
