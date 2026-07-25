// Package excel genera el archivo .xlsx exportable de los recibos procesados.
package excel

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"

	"rhix-backend/internal/receipt"
)

const sheetName = "Recibos"

var headers = []string{
	"RUC", "Razón social", "Serie-número", "Fecha de emisión",
	"Monto neto", "Retención", "Estado", "Email Message ID", "Creado",
}

// GenerateReceiptsXLSX arma un workbook con una fila por recibo.
func GenerateReceiptsXLSX(receipts []receipt.Receipt) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", sheetName); err != nil {
		return nil, fmt.Errorf("renombrando hoja: %w", err)
	}

	for col, header := range headers {
		cell, err := excelize.CoordinatesToCellName(col+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			return nil, err
		}
	}

	dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 14}) // dd/mm/yyyy
	if err != nil {
		return nil, fmt.Errorf("creando estilo de fecha: %w", err)
	}
	currencyStyle, err := f.NewStyle(&excelize.Style{NumFmt: 2}) // 0.00
	if err != nil {
		return nil, fmt.Errorf("creando estilo de moneda: %w", err)
	}

	for i, r := range receipts {
		row := i + 2

		f.SetCellValue(sheetName, cellRef(1, row), r.RUC)
		f.SetCellValue(sheetName, cellRef(2, row), r.RazonSocial)
		f.SetCellValue(sheetName, cellRef(3, row), r.SerieNumero)

		fechaCell := cellRef(4, row)
		f.SetCellValue(sheetName, fechaCell, r.FechaEmision)
		f.SetCellStyle(sheetName, fechaCell, fechaCell, dateStyle)

		montoCell := cellRef(5, row)
		f.SetCellValue(sheetName, montoCell, r.MontoNeto)
		f.SetCellStyle(sheetName, montoCell, montoCell, currencyStyle)

		retencionCell := cellRef(6, row)
		if r.Retencion != nil {
			f.SetCellValue(sheetName, retencionCell, *r.Retencion)
			f.SetCellStyle(sheetName, retencionCell, retencionCell, currencyStyle)
		}

		f.SetCellValue(sheetName, cellRef(7, row), string(r.Status))
		f.SetCellValue(sheetName, cellRef(8, row), r.EmailMessageID)

		creadoCell := cellRef(9, row)
		f.SetCellValue(sheetName, creadoCell, r.CreatedAt)
		f.SetCellStyle(sheetName, creadoCell, creadoCell, dateStyle)
	}

	for col := 1; col <= len(headers); col++ {
		colName, err := excelize.ColumnNumberToName(col)
		if err != nil {
			return nil, err
		}
		if err := f.SetColWidth(sheetName, colName, colName, 18); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("serializando xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

func cellRef(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}
