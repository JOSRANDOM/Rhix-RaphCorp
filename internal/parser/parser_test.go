package parser

import (
	"os"
	"testing"
	"time"
)

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("leyendo fixture %s: %v", name, err)
	}
	return data
}

func TestParse_ReciboConRetencion(t *testing.T) {
	parsed, err := Parse(mustReadFixture(t, "recibo_valido.xml"))
	if err != nil {
		t.Fatalf("Parse devolvió error inesperado: %v", err)
	}

	if parsed.RUC != "10012345678" {
		t.Errorf("RUC = %q, esperaba %q", parsed.RUC, "10012345678")
	}
	if parsed.RazonSocial != "JUAN PEREZ RAMIREZ" {
		t.Errorf("RazonSocial = %q, esperaba %q", parsed.RazonSocial, "JUAN PEREZ RAMIREZ")
	}
	if parsed.SerieNumero != "E001-123" {
		t.Errorf("SerieNumero = %q, esperaba %q", parsed.SerieNumero, "E001-123")
	}
	wantFecha := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	if !parsed.FechaEmision.Equal(wantFecha) {
		t.Errorf("FechaEmision = %v, esperaba %v", parsed.FechaEmision, wantFecha)
	}
	if parsed.MontoNeto != 920.00 {
		t.Errorf("MontoNeto = %v, esperaba %v", parsed.MontoNeto, 920.00)
	}
	if parsed.Retencion == nil {
		t.Fatal("Retencion = nil, esperaba 80.00")
	}
	if *parsed.Retencion != 80.00 {
		t.Errorf("Retencion = %v, esperaba %v", *parsed.Retencion, 80.00)
	}
}

func TestParse_ReciboSinRetencion(t *testing.T) {
	parsed, err := Parse(mustReadFixture(t, "recibo_sin_retencion.xml"))
	if err != nil {
		t.Fatalf("Parse devolvió error inesperado: %v", err)
	}

	if parsed.RUC != "10098765432" {
		t.Errorf("RUC = %q, esperaba %q", parsed.RUC, "10098765432")
	}
	if parsed.MontoNeto != 300.00 {
		t.Errorf("MontoNeto = %v, esperaba %v", parsed.MontoNeto, 300.00)
	}
	if parsed.Retencion != nil {
		t.Errorf("Retencion = %v, esperaba nil", *parsed.Retencion)
	}
}

func TestParse_XMLInvalido(t *testing.T) {
	_, err := Parse([]byte("esto no es XML"))
	if err == nil {
		t.Fatal("esperaba error con XML inválido, no hubo error")
	}
}

func TestParse_FaltaCampoRequerido(t *testing.T) {
	xml := []byte(`<?xml version="1.0"?>
<Invoice xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
         xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:ID>E001-999</cbc:ID>
</Invoice>`)

	_, err := Parse(xml)
	if err == nil {
		t.Fatal("esperaba error por falta de cbc:IssueDate, no hubo error")
	}
}
