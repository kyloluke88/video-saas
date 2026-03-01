package i18n

func init() {
	Register("en-US", map[string]string{
		"Email.required":           "Email is required",
		"Email.email":              "Invalid email format",
		"Email.email_not_exists":   "Email already exists",
		"Phone.required":           "Phone is required",
		"Phone.phone":              "Invalid phone number",
		"Phone.phone_not_exists":   "Phone already exists",
	})
}
