// Package parser extrae los datos de un Recibo por Honorarios Electrónico
// (UBL 2.1, InvoiceTypeCode "02") desde su XML adjunto.
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// Parsed son los campos de un recibo ya extraídos del XML.
type Parsed struct {
	RUC          string
	RazonSocial  string
	SerieNumero  string
	FechaEmision time.Time
	MontoNeto    float64
	Retencion    *float64
}

// Parse interpreta el XML UBL de un Recibo por Honorarios Electrónico.
func Parse(xmlBytes []byte) (*Parsed, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		return nil, fmt.Errorf("XML inválido: %w", err)
	}

	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("XML sin elemento raíz")
	}

	serieNumero := elementText(root, "cbc:ID")
	if serieNumero == "" {
		return nil, fmt.Errorf("falta cbc:ID (serie-número)")
	}

	fechaText := elementText(root, "cbc:IssueDate")
	if fechaText == "" {
		return nil, fmt.Errorf("falta cbc:IssueDate")
	}
	fecha, err := time.Parse("2006-01-02", fechaText)
	if err != nil {
		return nil, fmt.Errorf("cbc:IssueDate inválida %q: %w", fechaText, err)
	}

	supplier := root.FindElement("./cac:AccountingSupplierParty/cac:Party")
	if supplier == nil {
		return nil, fmt.Errorf("falta cac:AccountingSupplierParty/cac:Party")
	}

	ruc := elementText(supplier, "cac:PartyIdentification/cbc:ID")
	if ruc == "" {
		return nil, fmt.Errorf("falta el RUC del emisor (PartyIdentification/cbc:ID)")
	}

	razonSocial := elementText(supplier, "cac:PartyLegalEntity/cbc:RegistrationName")
	if razonSocial == "" {
		return nil, fmt.Errorf("falta la razón social del emisor (PartyLegalEntity/cbc:RegistrationName)")
	}

	montoText := elementText(root, "cac:LegalMonetaryTotal/cbc:PayableAmount")
	if montoText == "" {
		return nil, fmt.Errorf("falta cac:LegalMonetaryTotal/cbc:PayableAmount")
	}
	montoNeto, err := strconv.ParseFloat(montoText, 64)
	if err != nil {
		return nil, fmt.Errorf("PayableAmount inválido %q: %w", montoText, err)
	}

	retencion, err := extractRetencion(root)
	if err != nil {
		return nil, err
	}

	return &Parsed{
		RUC:          strings.TrimSpace(ruc),
		RazonSocial:  strings.TrimSpace(razonSocial),
		SerieNumero:  strings.TrimSpace(serieNumero),
		FechaEmision: fecha,
		MontoNeto:    montoNeto,
		Retencion:    retencion,
	}, nil
}

// extractRetencion suma los cac:AllowanceCharge que sean descuentos
// (ChargeIndicator=false) — así es como UBL representa la retención de
// renta de cuarta categoría en un RHE. Si no hay ninguno, no hubo retención.
func extractRetencion(root *etree.Element) (*float64, error) {
	var total float64
	found := false

	for _, ac := range root.FindElements("./cac:AllowanceCharge") {
		indicator := strings.TrimSpace(elementText(ac, "cbc:ChargeIndicator"))
		if !strings.EqualFold(indicator, "false") {
			continue
		}
		amountText := elementText(ac, "cbc:Amount")
		if amountText == "" {
			continue
		}
		amount, err := strconv.ParseFloat(amountText, 64)
		if err != nil {
			return nil, fmt.Errorf("AllowanceCharge/Amount inválido %q: %w", amountText, err)
		}
		total += amount
		found = true
	}

	if !found {
		return nil, nil
	}
	return &total, nil
}

func elementText(el *etree.Element, path string) string {
	found := el.FindElement("./" + path)
	if found == nil {
		return ""
	}
	return strings.TrimSpace(found.Text())
}
