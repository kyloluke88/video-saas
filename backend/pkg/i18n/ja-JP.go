package i18n

func init() {
	Register("ja-JP", map[string]string{
		"Email.required":           "メールアドレスは必須です",
		"Email.email":              "メール形式が正しくありません",
		"Email.email_not_exists":   "このメールは既に登録されています",
		"Phone.required":           "電話番号は必須です",
		"Phone.phone":              "電話番号の形式が正しくありません",
		"Phone.phone_not_exists":   "この電話番号は既に登録されています",
	})
}
