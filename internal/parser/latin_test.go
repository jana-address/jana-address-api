package parser

import "testing"

func TestCyrillic(t *testing.T) {
	for lat, want := range map[string]string{
		"Kabanbay batyra": "кабанбай батыра", "Zhandosova": "жандосова", "Yugo-Vostok": "юго-восток",
		"Satpayeva": "сатпаева", "Severnyy": "северный", "Shakhristan": "шахристан", "Zaschita": "защита",
		"Kobylandy": "кобыланды", "Oktyabrya": "октября",
	} {
		if got := cyrillic(lat); got != want {
			t.Errorf("cyrillic(%q) = %q, want %q", lat, got, want)
		}
	}
}
