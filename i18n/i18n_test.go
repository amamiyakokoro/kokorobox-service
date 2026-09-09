package i18n

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]Locale{
		"en":      English,
		"en-US":   English,
		"zh-TW":   TraditionalChinese,
		"zh_Hant": TraditionalChinese,
		"":        English,
	}
	for input, want := range tests {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFromAcceptLanguage(t *testing.T) {
	if got := FromAcceptLanguage("fr, zh-TW;q=0.9, en;q=0.8"); got != TraditionalChinese {
		t.Fatalf("FromAcceptLanguage() = %q, want %q", got, TraditionalChinese)
	}
}

func TestText(t *testing.T) {
	const message = "核心启动成功"
	if got, want := Text(English, message), "Core started successfully"; got != want {
		t.Errorf("English translation = %q, want %q", got, want)
	}
	if got, want := Text(TraditionalChinese, message), "核心啟動成功"; got != want {
		t.Errorf("Traditional Chinese translation = %q, want %q", got, want)
	}
}
