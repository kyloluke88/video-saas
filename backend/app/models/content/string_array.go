package content

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

type StringArray []string

func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case string:
		parsed, err := parsePostgresTextArray(v)
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	case []byte:
		parsed, err := parsePostgresTextArray(string(v))
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	case []string:
		*a = append((*a)[:0], v...)
		return nil
	default:
		return fmt.Errorf("unsupported StringArray scan type %T", src)
	}
}

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}

	parts := make([]string, 0, len(a))
	for _, item := range a {
		escaped := strings.ReplaceAll(item, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		parts = append(parts, `"`+escaped+`"`)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func parsePostgresTextArray(input string) (StringArray, error) {
	input = strings.TrimSpace(input)
	if input == "" || input == "{}" {
		return StringArray{}, nil
	}
	if len(input) < 2 || input[0] != '{' || input[len(input)-1] != '}' {
		return nil, fmt.Errorf("invalid postgres text array: %q", input)
	}

	var (
		result  = make(StringArray, 0, 8)
		buf     strings.Builder
		inQuote bool
		escape  bool
	)

	for i := 1; i < len(input)-1; i++ {
		ch := input[i]

		if escape {
			buf.WriteByte(ch)
			escape = false
			continue
		}

		switch ch {
		case '\\':
			escape = true
		case '"':
			inQuote = !inQuote
		case ',':
			if inQuote {
				buf.WriteByte(ch)
				continue
			}
			result = append(result, normalizePostgresArrayToken(buf.String()))
			buf.Reset()
		default:
			buf.WriteByte(ch)
		}
	}

	if escape || inQuote {
		return nil, fmt.Errorf("invalid postgres text array: %q", input)
	}

	result = append(result, normalizePostgresArrayToken(buf.String()))
	return result, nil
}

func normalizePostgresArrayToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "NULL" {
		return ""
	}
	return token
}
