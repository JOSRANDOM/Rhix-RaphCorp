package excel

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"rhix-backend/internal/receipt"
)

func TestGenerateReceiptsXLSX(t *testing.T) {
	retencion := 80.0
	receipts := []receipt.Receipt{
		{
			RUC:            "10012345678",
			RazonSocial:    "JUAN PEREZ RAMIREZ",
			SerieNumero:    "E001-123",
			FechaEmision:   time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
			MontoNeto:      920,
			Retencion:      &retencion,
			Status:         receipt.StatusProcessed,
			EmailMessageID: "msg-1",
			CreatedAt:      time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
		},
		{
			RUC:            "10098765432",
			RazonSocial:    "MARIA LOPEZ TORRES",
			SerieNumero:    "E001-124",
			FechaEmision:   time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC),
			MontoNeto:      300,
			Status:         receipt.StatusProcessed,
			EmailMessageID: "msg-2",
			CreatedAt:      time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC),
		},
	}

	data, err := GenerateReceiptsXLSX(receipts)
	if err != nil {
		t.Fatalf("GenerateReceiptsXLSX devolvió error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("GenerateReceiptsXLSX devolvió bytes vacíos")
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("el archivo generado no es un xlsx válido: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		t.Fatalf("leyendo hoja %q: %v", sheetName, err)
	}

	if len(rows) != 3 { // encabezado + 2 recibos
		t.Fatalf("esperaba 3 filas (encabezado + 2 recibos), obtuve %d", len(rows))
	}

	if rows[0][0] != "RUC" {
		t.Errorf("encabezado de la primera columna = %q, esperaba %q", rows[0][0], "RUC")
	}

	if rows[1][0] != "10012345678" {
		t.Errorf("fila 1 RUC = %q, esperaba %q", rows[1][0], "10012345678")
	}
	if rows[1][1] != "JUAN PEREZ RAMIREZ" {
		t.Errorf("fila 1 RazonSocial = %q, esperaba %q", rows[1][1], "JUAN PEREZ RAMIREZ")
	}

	if rows[2][5] != "" {
		t.Errorf("fila 2 (sin retención) columna Retención = %q, esperaba vacía", rows[2][5])
	}
}
