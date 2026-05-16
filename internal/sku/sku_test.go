package sku_test

import (
	"testing"

	"github.com/norfrig/hub/internal/sku"
)

func TestPrimary(t *testing.T) {
	e := sku.New(nil)

	cases := []struct {
		name    string
		path    string
		wantSKU string
		wantSrc string
	}{
		// --- path-first ---
		{
			"numeric SKU de carpeta raíz",
			`C:\testdata\1013\1013_1.jpg`,
			"1013", "path",
		},
		{
			"SKU + subcarpeta JPG — JPG sin dígitos, se ignora",
			`C:\testdata\1467\JPG\1467_ (1).jpg`,
			"1467", "path",
		},
		{
			"SKU + subcarpeta PNG — PNG sin dígitos, se ignora",
			`C:\testdata\1467\PNG\1467_ (2).jpg`,
			"1467", "path",
		},
		{
			"SKU + subcarpeta WEB — WEB sin dígitos, se ignora",
			`C:\testdata\1467\WEB\1467_ (3).jpg`,
			"1467", "path",
		},
		{
			"SKU alfanumérico 3EU107",
			`C:\testdata\3EU107\3EU107_1.jpg`,
			"3EU107", "path",
		},
		{
			"código COM0039",
			`C:\testdata\COM0039\JPG\COM0039_ (1).jpg`,
			"COM0039", "path",
		},
		{
			"SKU largo WW01F02611",
			`C:\testdata\WW01F02611\JPG\WW01F02611_ (1).jpg`,
			"WW01F02611", "path",
		},
		{
			"nombre de carpeta con espacio — primer token es el SKU",
			`C:\testdata\3EU314 2\foto.jpg`,
			"3EU314", "path",
		},
		{
			"SKU 76DXR",
			`C:\testdata\76DXR\foto.jpg`,
			"76DXR", "path",
		},
		{
			"SKU C00111416",
			`C:\testdata\C00111416\foto.jpg`,
			"C00111416", "path",
		},
		{
			"SKU con guión 632F15-1",
			`C:\testdata\632F15-1\foto.jpg`,
			"632F15-1", "path",
		},
		{
			"código de barras 13 dígitos",
			`C:\testdata\2944745031074\foto.jpg`,
			"2944745031074", "path",
		},
		{
			"ruta real LAUTARO PUBLICACIONES COM0531 JPG",
			`M:\LAUTARO\PUBLICACIONES\COM0531\JPG\foto.jpg`,
			"COM0531", "path",
		},
		{
			"ruta real con carpetas ruidosas — Recursos antiguos, FOTOS VARIAS",
			`M:\RESPALDOS\Recursos antiguos (DESORDENADOS)\FOTOS VARIAS\3EU107\foto.jpg`,
			"3EU107", "path",
		},
		{
			"carpeta más profunda con SKU gana sobre carpeta más alta",
			`M:\LAUTARO\1013\3EU107\JPG\foto.jpg`,
			"3EU107", "path",
		},
		{
			"SKU 27H00416",
			`C:\testdata\27H00416\foto.jpg`,
			"27H00416", "path",
		},
		{
			"path gana sobre nombre de archivo — archivo NUEVO.jpg en carpeta 1013",
			`C:\testdata\1013\NUEVO.jpg`,
			"1013", "path",
		},
		{
			"archivo con nombre 1013_.jpg — underscore al final",
			`C:\testdata\1013\1013_.jpg`,
			"1013", "path",
		},
		{
			"ruta con COM0262 y PNG",
			`M:\PABLO_Recursos\PUBLICACIONES\COM0262\PNG\COM0262_ (2).png`,
			"COM0262", "path",
		},
		// --- filename fallback ---
		{
			"fallback a nombre: prefijo lowercase con underscore",
			`nosku\path\3eu102_01.jpg`,
			"3EU102", "filename",
		},
		{
			"fallback a nombre: código COM en filename",
			`sin\sku\COM0531_ (1).jpg`,
			"COM0531", "filename",
		},
		{
			"fallback a nombre: prefijo antes de espacio y paréntesis",
			`sin\sku\3EU315 (1).jpg`,
			"3EU315", "filename",
		},
		// --- sin SKU ---
		{
			"sin SKU — carpetas y archivo sin dígitos",
			`FOTOS\VARIAS\NUEVO.jpg`,
			"", "",
		},
		{
			"sin SKU — foto.jpg genérica en ruta sin SKU",
			`nosku\path\foto.jpg`,
			"", "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := e.Primary(tc.path)
			if r.SKU != tc.wantSKU {
				t.Errorf("SKU = %q, want %q (path: %s)", r.SKU, tc.wantSKU, tc.path)
			}
			if r.Source != tc.wantSrc {
				t.Errorf("Source = %q, want %q (path: %s)", r.Source, tc.wantSrc, tc.path)
			}
		})
	}
}

func TestExtractMultiple(t *testing.T) {
	e := sku.New(nil)

	// Verifica que Extract devuelve múltiples candidatos si hay más de un SKU en el path
	results := e.Extract(`M:\LAUTARO\1013\3EU107\JPG\foto.jpg`)
	if len(results) < 2 {
		t.Fatalf("esperaba al menos 2 resultados, obtuve %d", len(results))
	}
	if results[0].SKU != "3EU107" {
		t.Errorf("primer SKU = %q, want 3EU107", results[0].SKU)
	}
	if results[1].SKU != "1013" {
		t.Errorf("segundo SKU = %q, want 1013", results[1].SKU)
	}
}

func TestNormalization(t *testing.T) {
	e := sku.New(nil)

	// SKUs siempre deben salir en mayúsculas
	r := e.Primary(`C:\testdata\3eu107\foto.jpg`)
	if r.SKU != "3EU107" {
		t.Errorf("esperaba 3EU107 (uppercase), obtuve %q", r.SKU)
	}
}

func TestCustomBlacklist(t *testing.T) {
	e := sku.New([]string{"PUBLICACIONES", "RECURSOS"})

	r := e.Primary(`M:\RECURSOS\PUBLICACIONES\COM0039\foto.jpg`)
	if r.SKU != "COM0039" {
		t.Errorf("esperaba COM0039, obtuve %q", r.SKU)
	}
}
