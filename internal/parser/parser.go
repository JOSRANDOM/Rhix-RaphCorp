// Package parser extrae los datos de un Recibo por Honorarios Electrónico
// (UBL 2.1, InvoiceTypeCode "01"/"02" según el emisor) desde su XML adjunto.
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
//
// La estructura no está 100% estandarizada entre proveedores: algunos ponen
// el RUC/razón social del emisor en cac:Party/cac:PartyIdentification +
// PartyLegalEntity, otros (como el que emite SUNAT/Llama.pe) lo ponen en
// cbc:CustomerAssignedAccountID directo bajo AccountingSupplierParty y en
// cac:Party/cac:PartyName. Probamos ambas formas.
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

	supplierParty := root.FindElement("./cac:AccountingSupplierParty")
	if supplierParty == nil {
		return nil, fmt.Errorf("falta cac:AccountingSupplierParty")
	}

	ruc := elementText(supplierParty, "cbc:CustomerAssignedAccountID")
	if ruc == "" {
		ruc = elementText(supplierParty, "cac:Party/cac:PartyIdentification/cbc:ID")
	}
	if ruc == "" {
		return nil, fmt.Errorf("falta el RUC del emisor")
	}

	razonSocial := elementText(supplierParty, "cac:Party/cac:PartyName/cbc:Name")
	if razonSocial == "" {
		razonSocial = elementText(supplierParty, "cac:Party/cac:PartyLegalEntity/cbc:RegistrationName")
	}
	if razonSocial == "" {
		return nil, fmt.Errorf("falta la razón social del emisor")
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

// extractRetencion busca el monto de retención de renta de cuarta categoría.
// Hay dos formas de representarlo en la práctica:
//  1. cac:AllowanceCharge con ChargeIndicator=false (un descuento aplicado
//     sobre el total).
//  2. Un cac:TaxSubtotal (a nivel de documento o de cada InvoiceLine) cuya
//     TaxCategory tiene un ID que contiene "RET" (p. ej. "RET 4TA"), con el
//     monto en cbc:TaxAmount.
//
// Devuelve nil si no se encuentra ninguna de las dos formas (no hubo
// retención en el documento).
func extractRetencion(root *etree.Element) (*float64, error) {
	if amount, found, err := retencionFromAllowanceCharge(root); err != nil {
		return nil, err
	} else if found {
		return &amount, nil
	}

	if amount, found, err := retencionFromTaxSubtotals(root); err != nil {
		return nil, err
	} else if found {
		return &amount, nil
	}

	return nil, nil
}

func retencionFromAllowanceCharge(root *etree.Element) (float64, bool, error) {
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
			return 0, false, fmt.Errorf("AllowanceCharge/Amount inválido %q: %w", amountText, err)
		}
		total += amount
		found = true
	}

	return total, found, nil
}

func retencionFromTaxSubtotals(root *etree.Element) (float64, bool, error) {
	var subtotals []*etree.Element
	subtotals = append(subtotals, root.FindElements("./cac:TaxTotal/cac:TaxSubtotal")...)
	for _, line := range root.FindElements("./cac:InvoiceLine") {
		subtotals = append(subtotals, line.FindElements("./cac:TaxTotal/cac:TaxSubtotal")...)
	}

	var total float64
	found := false

	for _, sub := range subtotals {
		categoryID := elementText(sub, "cac:TaxCategory/cbc:ID")
		if !strings.Contains(strings.ToUpper(categoryID), "RET") {
			continue
		}
		amountText := elementText(sub, "cbc:TaxAmount")
		if amountText == "" {
			continue
		}
		amount, err := strconv.ParseFloat(amountText, 64)
		if err != nil {
			return 0, false, fmt.Errorf("TaxSubtotal/TaxAmount inválido %q: %w", amountText, err)
		}
		total += amount
		found = true
	}

	return total, found, nil
}

func elementText(el *etree.Element, path string) string {
	found := el.FindElement("./" + path)
	if found == nil {
		return ""
	}
	return strings.TrimSpace(found.Text())
}
