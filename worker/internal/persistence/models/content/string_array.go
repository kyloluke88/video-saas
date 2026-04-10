package content

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

type StringArray []string

func (a *StringArray) Scan(value interface{}) error {
	switch raw := value.(type) {
	case nil:
		*a = StringArray{}
		return nil
	case string:
		parsed, err := parsePGTextArray(raw)
		if err != nil {
			return err
		}
		*a = StringArray(parsed)
		return nil
	case []byte:
		parsed, err := parsePGTextArray(string(raw))
		if err != nil {
			return err
		}
		*a = StringArray(parsed)
		return nil
	default:
		return fmt.Errorf("unsupported StringArray scan type %T", value)
	}
}

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}

	parts := make([]string, 0, len(a))
	for _, value := range a {
		escaped := strings.ReplaceAll(value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		parts = append(parts, `"`+escaped+`"`)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func parsePGTextArray(input string) ([]string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed == "{}" {
		return []string{}, nil
	}
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return nil, fmt.Errorf("invalid postgres text array: %q", input)
	}

	body := trimmed[1 : len(trimmed)-1]
	if body == "" {
		return []string{}, nil
	}

	result := make([]string, 0)
	var current strings.Builder
	inQuotes := false
	escaped := false

	for i := 0; i < len(body); i++ {
		ch := body[i]
		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		switch ch {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
		case ',':
			if inQuotes {
				current.WriteByte(ch)
				continue
			}
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}

	if inQuotes || escaped {
		return nil, fmt.Errorf("invalid postgres text array quoting: %q", input)
	}

	result = append(result, current.String())
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
	}
	return result, nil
}
