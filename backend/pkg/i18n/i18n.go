package i18n

// messages 保存所有语言的翻译
var messages = map[string]map[string]string{}

// Register 注册语言包
func Register(lang string, data map[string]string) {
	messages[lang] = data
}


// T 翻译函数，找不到则 fallback en-US
func T(lang string, key string) string {
	if m, ok := messages[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}

	if m, ok := messages["en-US"]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}

	return key
}
