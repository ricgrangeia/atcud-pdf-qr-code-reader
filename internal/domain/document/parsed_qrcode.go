package document

// ParsedQRCode é um QR code fiscal português descodificado campo a campo.
// Todos os campos estão sempre presentes na resposta JSON, mesmo que vazios,
// para que o consumidor saiba exactamente o que estava (ou não estava) no QR.
type ParsedQRCode struct {
	NumeroPagina int    `json:"numero_pagina"`
	ConteudoBruto string `json:"conteudo_bruto"`

	// A — NIF do Emitente (Vendedor)
	Emitente EmitenteInfo `json:"emitente"`

	// B, C — NIF e País do Adquirente (Comprador)
	Adquirente AdquirenteInfo `json:"adquirente"`

	// D, E, F, G, H — Dados do documento fiscal
	Documento DocumentoInfo `json:"documento"`

	// I/J/K — Linhas de imposto por região e taxa
	Impostos ImpostosInfo `json:"impostos"`

	// O — Total do documento
	Totais TotaisInfo `json:"totais"`

	// Q — 4 caracteres da assinatura cifrada impressa no documento
	CaracteresAssinatura string `json:"caracteres_assinatura"`

	// R — Número do certificado do software de faturação
	NumeroCertificado string `json:"numero_certificado"`

	// S — Informações adicionais (ex: meio de pagamento "MB;ENTIDADE;REFERENCIA;VALOR")
	InformacoesAdicionais string `json:"informacoes_adicionais"`
}

// EmitenteInfo identifica o vendedor.
type EmitenteInfo struct {
	// Descricao é o nome da entidade resolvido pelo serviço de NIF (opcional).
	Descricao string `json:"descricao,omitempty"`
	// A — NIF do emitente
	NIF string `json:"nif"`
}

// AdquirenteInfo identifica o comprador.
type AdquirenteInfo struct {
	// Descricao é o nome da entidade resolvido pelo serviço de NIF (opcional).
	Descricao string `json:"descricao,omitempty"`
	// B — NIF do adquirente (ou "0" se consumidor final)
	NIF string `json:"nif"`
	// C — País do adquirente (código ISO 3166-1 alpha-2)
	Pais string `json:"pais"`
}

// DocumentoInfo descreve o documento fiscal.
type DocumentoInfo struct {
	// D — Código do tipo de documento
	TipoCodigo string `json:"tipo_codigo"`
	// D — Designação oficial do tipo de documento
	Tipo string `json:"tipo"`

	// E — Código do estado do documento
	EstadoCodigo string `json:"estado_codigo"`
	// E — Designação do estado do documento
	Estado string `json:"estado"`

	// F — Data do documento no formato AAAA-MM-DD
	Data string `json:"data"`

	// G — Identificador único do documento (ex: "FT 20260/00659283")
	Identificador string `json:"identificador"`

	// H — ATCUD: código de validação + número sequencial (ex: "J66S9FDD-659283")
	ATCUD string `json:"atcud"`
}

// ImpostosInfo agrega todas as linhas de imposto e os totais.
type ImpostosInfo struct {
	// Linhas de base tributável e IVA, por região e taxa
	Linhas []LinhaImposto `json:"linhas"`

	// N — Total de imposto (IVA + Imposto do Selo)
	TotalImposto float64 `json:"total_imposto"`

	// P — Retenção na fonte (quando aplicável)
	RetencaoFonte float64 `json:"retencao_fonte"`
}

// LinhaImposto representa uma linha de tributação:
// uma combinação de região fiscal, taxa e respectivos valores.
type LinhaImposto struct {
	// Região fiscal: "Portugal Continental", "Açores" ou "Madeira"
	Regiao string `json:"regiao"`
	// Taxa aplicada: "Isento", "Taxa Reduzida", "Taxa Intermédia" ou "Taxa Normal"
	Taxa string `json:"taxa"`
	// Base tributável sujeita a esta taxa
	BaseTributavel float64 `json:"base_tributavel"`
	// Valor do IVA calculado sobre a base tributável
	ValorIVA float64 `json:"valor_iva"`
}

// TotaisInfo contém os totais monetários do documento.
type TotaisInfo struct {
	// O — Total do documento incluindo todos os impostos (valor a pagar)
	TotalDocumento float64 `json:"total_documento"`
}
