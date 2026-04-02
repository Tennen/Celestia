package client

// AccountConfig holds the credentials and session state for a single Petkit account.
type AccountConfig struct {
	Name             string `json:"name,omitempty"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Region           string `json:"region"`
	Timezone         string `json:"timezone,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	SessionUserID    string `json:"session_user_id,omitempty"`
	SessionCreatedAt string `json:"session_created_at,omitempty"`
	SessionExpiresAt string `json:"session_expires_at,omitempty"`
	SessionBaseURL   string `json:"session_base_url,omitempty"`
}

// CompatConfig holds optional Petkit cloud compatibility overrides.
type CompatConfig struct {
	PassportBaseURL string `json:"passport_base_url,omitempty"`
	ChinaBaseURL    string `json:"china_base_url,omitempty"`
	APIVersion      string `json:"api_version,omitempty"`
	ClientHeader    string `json:"client_header,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
	Locale          string `json:"locale,omitempty"`
	AcceptLanguage  string `json:"accept_language,omitempty"`
	Platform        string `json:"platform,omitempty"`
	OSVersion       string `json:"os_version,omitempty"`
	ModelName       string `json:"model_name,omitempty"`
	PhoneBrand      string `json:"phone_brand,omitempty"`
	Source          string `json:"source,omitempty"`
	HourMode        string `json:"hour_mode,omitempty"`
}

// DefaultCompatConfig returns the built-in Petkit cloud compatibility defaults.
func DefaultCompatConfig() CompatConfig {
	return CompatConfig{
		PassportBaseURL: "https://passport.petkt.com/",
		ChinaBaseURL:    "https://api.petkit.cn/6/",
		APIVersion:      "12.4.9",
		ClientHeader:    "android(15.1;23127PN0CG)",
		UserAgent:       "okhttp/3.14.19",
		Locale:          "en-US",
		AcceptLanguage:  "en-US;q=1, it-US;q=0.9",
		Platform:        "android",
		OSVersion:       "15.1",
		ModelName:       "23127PN0CG",
		PhoneBrand:      "Xiaomi",
		Source:          "app.petkit-android",
		HourMode:        "24",
	}
}
