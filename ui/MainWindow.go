package ui

import (
	"excel-splitter/services"
	"fmt"
	"image/color"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
)

type MainWindow struct {
	window             *app.Window
	theme              *material.Theme
	ops                op.Ops
	exp                *explorer.Explorer
	excelService       *services.ExcelService
	disabled           bool
	sheets             []string
	currentSheetIndex  int
	columns            []string
	currentColumnIndex int
	status             string
	subStatus          string

	openButton    widget.Clickable
	sheetList     widget.List
	sheetButtons  []widget.Clickable
	columnList    widget.List
	columnButtons []widget.Clickable
}

func NewMainWindow(excelService *services.ExcelService) *MainWindow {
	window := new(app.Window)
	window.Option(app.Title("Excel Splitter"), app.Size(600, 250))
	theme := material.NewTheme()
	exp := explorer.NewExplorer(window)
	mainWindow := &MainWindow{
		window:            window,
		theme:             theme,
		exp:               exp,
		excelService:      excelService,
		currentSheetIndex: -1,
		status:            "Waiting for input",
	}
	excelService.RegisterStatusCallback("MainWindow", func(subStatus string) {
		mainWindow.subStatus = subStatus
		mainWindow.window.Invalidate()
	})
	return mainWindow
}

func (mainWindow *MainWindow) resetSheets() {
	mainWindow.sheets = []string{}
	mainWindow.currentSheetIndex = -1
	mainWindow.sheetButtons = make([]widget.Clickable, len(mainWindow.sheets))
}

func (mainWindow *MainWindow) resetColumns() {
	mainWindow.columns = []string{}
	mainWindow.currentColumnIndex = -1
	mainWindow.columnButtons = make([]widget.Clickable, len(mainWindow.columns))
}

func (mainWindow *MainWindow) setError(err error) {
	mainWindow.status = fmt.Sprintf("Error: %s", err)
}

func (mainWindow *MainWindow) Run() {
	for {
		switch e := mainWindow.window.Event().(type) {
		case app.DestroyEvent:
			mainWindow.excelService.DisposeFileIfLoaded()
			return
		case app.FrameEvent:
			ctx := app.NewContext(&mainWindow.ops, e)
			err := mainWindow.handleEvents(ctx)
			if err != nil {
				mainWindow.setError(err)
			}
			if mainWindow.disabled {
				ctx = ctx.Disabled()
			}
			mainWindow.draw(ctx)
			e.Frame(ctx.Ops)
		}
	}
}

func (mainWindow *MainWindow) handleEvents(ctx layout.Context) error {
	if mainWindow.openButton.Clicked(ctx) {
		err := mainWindow.excelService.DisposeFileIfLoaded()
		if err != nil {
			return err
		}
		mainWindow.resetSheets()
		mainWindow.resetColumns()
		file, err := mainWindow.exp.ChooseFile("xlsx")
		if err != nil {
			if err == explorer.ErrUserDecline {
				mainWindow.status = "Waiting for input"
				return nil
			}
			return err
		}
		err = mainWindow.excelService.LoadFile(file)
		if err != nil {
			return err
		}
		mainWindow.sheets = mainWindow.excelService.GetSheets()
		mainWindow.sheetButtons = make([]widget.Clickable, len(mainWindow.sheets))
		mainWindow.status = "Excel file selected"
	}
	for i := range mainWindow.sheetButtons {
		if mainWindow.sheetButtons[i].Clicked(ctx) {
			mainWindow.resetColumns()
			mainWindow.currentSheetIndex = i
			sheet := mainWindow.sheets[i]
			columns, err := mainWindow.excelService.GetColumns(sheet)
			if err != nil {
				return err
			}
			mainWindow.columns = columns
			mainWindow.columnButtons = make([]widget.Clickable, len(mainWindow.columns))
			mainWindow.status = fmt.Sprintf(`Sheet "%s" selected`, sheet)
		}
	}
	for i := range mainWindow.columnButtons {
		if mainWindow.columnButtons[i].Clicked(ctx) {
			mainWindow.currentColumnIndex = i
			sheet := mainWindow.sheets[mainWindow.currentSheetIndex]
			mainWindow.disabled = true
			mainWindow.status = fmt.Sprintf(`Splitting sheet "%s" based on column "%s"`, sheet, mainWindow.columns[i])
			go func(i int) {
				start := time.Now()
				err := mainWindow.excelService.SplitByColumn(sheet, i)
				elapsed := time.Since(start)
				if err != nil {
					mainWindow.setError(err)
				} else {
					mainWindow.status = fmt.Sprintf(`Finished splitting sheet "%s" based on column "%s" (took %.2fs)`, sheet, mainWindow.columns[i], elapsed.Seconds())
				}
				mainWindow.subStatus = ""
				mainWindow.disabled = false
				mainWindow.window.Invalidate()
			}(i)
		}
	}
	return nil
}

func (mainWindow *MainWindow) draw(ctx layout.Context) {
	status := mainWindow.status
	if len(mainWindow.subStatus) > 0 {
		status = fmt.Sprintf("%s\n%s", status, mainWindow.subStatus)
	}
	layout.UniformInset(unit.Dp(15)).Layout(
		ctx,
		func(ctx layout.Context) layout.Dimensions {
			return layout.Flex{
				Axis:    layout.Vertical,
				Spacing: layout.SpaceBetween,
			}.Layout(
				ctx,
				layout.Rigid(func(ctx layout.Context) layout.Dimensions {
					return layout.Flex{
						Axis: layout.Vertical,
					}.Layout(
						ctx,
						layout.Rigid(material.Button(mainWindow.theme, &mainWindow.openButton, "Browse").Layout),
						layout.Rigid(layout.Spacer{Height: 10}.Layout),
						layout.Rigid(func(ctx layout.Context) layout.Dimensions {
							return material.List(mainWindow.theme, &mainWindow.sheetList).Layout(ctx, len(mainWindow.sheets), func(ctx layout.Context, i int) layout.Dimensions {
								button := material.Button(mainWindow.theme, &mainWindow.sheetButtons[i], mainWindow.sheets[i])
								if i == mainWindow.currentSheetIndex {
									button.Background = color.NRGBA{A: 0xff, R: 0x33, G: 0x99, B: 0x33}
								}
								return layout.Inset{Right: unit.Dp(10)}.Layout(ctx, button.Layout)
							})
						}),
						layout.Rigid(func(ctx layout.Context) layout.Dimensions {
							return material.List(mainWindow.theme, &mainWindow.columnList).Layout(ctx, len(mainWindow.columns), func(ctx layout.Context, i int) layout.Dimensions {
								button := material.Button(mainWindow.theme, &mainWindow.columnButtons[i], mainWindow.columns[i])
								if i == mainWindow.currentColumnIndex {
									button.Background = color.NRGBA{A: 0xff, R: 0x33, G: 0x99, B: 0x33}
								}
								return layout.Inset{Right: unit.Dp(10)}.Layout(ctx, button.Layout)
							})
						}),
					)
				}),
				layout.Rigid(material.Label(mainWindow.theme, unit.Sp(15), status).Layout),
			)
		},
	)
}
