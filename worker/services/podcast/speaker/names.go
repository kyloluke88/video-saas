package podcastspeaker

import "strings"

func NormalizeRole(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "female", "f", "woman", "girl", "女":
		return "female"
	case "male", "m", "man", "boy", "男":
		return "male"
	}
	if strings.Contains(normalized, "female") || strings.Contains(normalized, "女") {
		return "female"
	}
	return "male"
}

func PreferredDisplayName(providedName string) string {
	providedName = strings.TrimSpace(providedName)
	if providedName != "" && !isGenericRoleAlias(providedName) {
		return providedName
	}
	return ""
}

func IsJapaneseLanguage(language string) bool {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "ja", "ja-jp":
		return true
	default:
		return false
	}
}

func isGenericRoleAlias(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "female", "male", "f", "m", "woman", "man", "girl", "boy", "女", "男":
		return true
	default:
		return false
	}
}
