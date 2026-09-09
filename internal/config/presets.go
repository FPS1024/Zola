package config

func Preset(id string) (Provider, bool) {
	switch id {
	case "deepseek":
		return Provider{
			ID:      "deepseek",
			Name:    "DeepSeek",
			BaseURL: "https://api.deepseek.com",
			Model:   "deepseek-v4-flash",
			WireAPI: WireAPIResponses,
			EnvKey:  "DEEPSEEK_API_KEY",
			Enabled: true,
		}, true
	case "openai":
		return Provider{
			ID:      "openai",
			Name:    "OpenAI",
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-5",
			WireAPI: WireAPIResponses,
			EnvKey:  "OPENAI_API_KEY",
			Enabled: true,
		}, true
	case "openrouter":
		return Provider{
			ID:      "openrouter",
			Name:    "OpenRouter",
			BaseURL: "https://openrouter.ai/api/v1",
			Model:   "openrouter/auto",
			WireAPI: WireAPIChat,
			EnvKey:  "OPENROUTER_API_KEY",
			Enabled: true,
		}, true
	default:
		return Provider{}, false
	}
}
