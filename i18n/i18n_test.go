package i18n

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]Locale{
		"en":      English,
		"en-US":   English,
		"zh-CN":   SimplifiedChinese,
		"zh_Hans": SimplifiedChinese,
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
	const message = "Core started successfully"
	if got, want := Text(English, message), message; got != want {
		t.Errorf("English translation = %q, want %q", got, want)
	}
	if got, want := Text(SimplifiedChinese, message), "核心启动成功"; got != want {
		t.Errorf("Simplified Chinese translation = %q, want %q", got, want)
	}
	if got, want := Text(TraditionalChinese, message), "核心啟動成功"; got != want {
		t.Errorf("Traditional Chinese translation = %q, want %q", got, want)
	}
}

func TestTextFallsBackToEnglish(t *testing.T) {
	const message = "Untranslated English source"
	if got := Text(TraditionalChinese, message); got != message {
		t.Errorf("translation fallback = %q, want %q", got, message)
	}
}
