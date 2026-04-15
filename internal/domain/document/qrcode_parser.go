package document

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseQRCode descodifica a string bruta de um QR code fiscal português
// e devolve um ParsedQRCode com todos os campos preenchidos.
//
// Formato AT: pares chave:valor separados por "*", ex: "A:500906840*B:0*...".
// Referência: Portaria n.º 195/2020 e documentação técnica da AT.
//
// Mapeamento de campos:
//
//	A  — NIF do emitente
//	B  — NIF do adquirente
//	C  — País do adquirente
//	D  — Tipo de documento
//	E  — Estado do documento
//	F  — Data (AAAAMMDD)
//	G  — Identificador único do documento
//	H  — ATCUD
//	I1 — Espaço fiscal (PT = Portugal Continental)
//	I2 — Base tributável isenta de IVA
//	I3 — Base tributável à taxa reduzida
//	I4 — Total de IVA à taxa reduzida
//	I5 — Base tributável à taxa intermédia
//	I6 — Total de IVA à taxa intermédia
//	I7 — Base tributável à taxa normal
//	I8 — Total de IVA à taxa normal
//	J1-J8 — Açores (mesma estrutura)
//	K1-K8 — Madeira (mesma estrutura)
//	N  — Total de imposto (IVA + Imposto do Selo)
//	O  — Total do documento (valor a pagar)
//	P  — Retenção na fonte
//	Q  — 4 caracteres da assinatura cifrada
//	R  — Número do certificado do software
//	S  — Informações adicionais (ex: meio de pagamento)
func ParseQRCode(content string) (*ParsedQRCode, error) {
	campos := extractFields(content)

	p := &ParsedQRCode{ConteudoBruto: content}

	// ── Emitente e Adquirente ─────────────────────────────────
	p.Emitente.NIF = campos["A"]
	p.Adquirente.NIF = campos["B"]
	p.Adquirente.Pais = campos["C"]

	// ── Documento ─────────────────────────────────────────────
	p.Documento.TipoCodigo = campos["D"]
	p.Documento.Tipo = nameTypeDocument(campos["D"])
	p.Documento.EstadoCodigo = campos["E"]
	p.Documento.Estado = nameStateDocument(campos["E"])
	p.Documento.Data = FormatDate(campos["F"])
	p.Documento.Identificador = campos["G"]
	p.Documento.ATCUD = campos["H"]

	// ── Impostos ──────────────────────────────────────────────
	// Regiões fiscais: I = Portugal Continental, J = Açores, K = Madeira.
	// Estrutura por prefixo:
	//   x1 = identificador da região
	//   x2 = base isenta
	//   x3 = base taxa reduzida  /  x4 = IVA taxa reduzida
	//   x5 = base taxa intermédia / x6 = IVA taxa intermédia
	//   x7 = base taxa normal     / x8 = IVA taxa normal
	p.Impostos.Linhas = extractTaxesLines(campos)
	p.Impostos.TotalImposto = parseMontante(campos["N"])
	p.Impostos.RetencaoFonte = parseMontante(campos["P"])

	// ── Totais ────────────────────────────────────────────────
	p.Totais.TotalDocumento = parseMontante(campos["O"])

	// ── Outros ────────────────────────────────────────────────
	p.CaracteresAssinatura = campos["Q"]
	p.NumeroCertificado = campos["R"]
	p.InformacoesAdicionais = campos["S"]

	if p.Emitente.NIF == "" {
		return nil, fmt.Errorf("o QR code não contém NIF do emitente (campo A) — pode não ser um documento fiscal")
	}

	return p, nil
}

// extractFields divide "A:val*B:val*..." num map[chave]valor.
func extractFields(conteudo string) map[string]string {
	out := make(map[string]string)
	for _, par := range strings.Split(conteudo, "*") {
		idx := strings.Index(par, ":")
		if idx < 0 {
			continue
		}
		out[par[:idx]] = par[idx+1:]
	}
	return out
}

// extractTaxesLines lê os campos de imposto para as três regiões fiscais.
func extractTaxesLines(campos map[string]string) []LinhaImposto {
	regioes := []struct {
		prefixo string
		nome    string
	}{
		{"I", "Portugal Continental"},
		{"J", "Açores"},
		{"K", "Madeira"},
	}

	linhas := make([]LinhaImposto, 0)

	for _, r := range regioes {
		// Ignorar região se o marcador de espaço fiscal não estiver presente.
		if _, existe := campos[r.prefixo+"1"]; !existe {
			continue
		}

		// Base isenta (sem IVA associado).
		if base := parseMontante(campos[r.prefixo+"2"]); base > 0 {
			linhas = append(linhas, LinhaImposto{
				Regiao:         r.nome,
				Taxa:           "Isento",
				BaseTributavel: base,
				ValorIVA:       0,
			})
		}

		// Taxa reduzida.
		if base := parseMontante(campos[r.prefixo+"3"]); base > 0 {
			linhas = append(linhas, LinhaImposto{
				Regiao:         r.nome,
				Taxa:           "Taxa Reduzida",
				BaseTributavel: base,
				ValorIVA:       parseMontante(campos[r.prefixo+"4"]),
			})
		}

		// Taxa intermédia.
		if base := parseMontante(campos[r.prefixo+"5"]); base > 0 {
			linhas = append(linhas, LinhaImposto{
				Regiao:         r.nome,
				Taxa:           "Taxa Intermédia",
				BaseTributavel: base,
				ValorIVA:       parseMontante(campos[r.prefixo+"6"]),
			})
		}

		// Taxa normal.
		if base := parseMontante(campos[r.prefixo+"7"]); base > 0 {
			linhas = append(linhas, LinhaImposto{
				Regiao:         r.nome,
				Taxa:           "Taxa Normal",
				BaseTributavel: base,
				ValorIVA:       parseMontante(campos[r.prefixo+"8"]),
			})
		}
	}

	return linhas
}

// FormatDate converte AAAAMMDD em AAAA-MM-DD.
func FormatDate(bruto string) string {
	if len(bruto) != 8 {
		return bruto
	}
	return bruto[:4] + "-" + bruto[4:6] + "-" + bruto[6:]
}

// parseMontante converte uma string decimal (ponto como separador) em float64.
func parseMontante(bruto string) float64 {
	if bruto == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(bruto, 64)
	return v
}

// nameTypeDocument devolve a designação oficial portuguesa para o código AT.
func nameTypeDocument(code string) string {
	nomes := map[string]string{
		"FT": "Fatura",
		"FR": "Fatura-Recibo",
		"FS": "Fatura Simplificada",
		"ND": "Nota de Débito",
		"NC": "Nota de Crédito",
		"GR": "Guia de Remessa",
		"GT": "Guia de Transporte",
		"GD": "Guia ou Nota de Devolução",
		"RG": "Recibo",
		"RC": "Recibo IVA de Caixa",
		"CM": "Consulta de Mesa",
		"PF": "Fatura Pró-Forma",
		"OR": "Orçamento",
		"NE": "Nota de Encomenda",
	}
	if nome, ok := nomes[code]; ok {
		return nome
	}
	return code
}

// nameStateDocument devolve a designação oficial portuguesa para o código AT.
func nameStateDocument(code string) string {
	nomes := map[string]string{
		"N": "Normal",
		"S": "Autofaturação",
		"C": "Cancelado",
		"A": "Anulado",
		"R": "Documento Sumário",
		"F": "Faturado",
	}
	if nome, ok := nomes[code]; ok {
		return nome
	}
	return code
}
