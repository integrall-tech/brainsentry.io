package service

import (
	"regexp"
	"strings"
)

// PrivacyStrippingService removes sensitive data entirely from content before storage,
// complementing PIIService which only masks. Stripping is more aggressive — data is gone.
type PrivacyStrippingService struct {
	pii             *PIIService
	privateTagRegex *regexp.Regexp
	envVarRegex     *regexp.Regexp
	secretPatterns  []*regexp.Regexp
}

// NewPrivacyStrippingService creates a new PrivacyStrippingService.
func NewPrivacyStrippingService() *PrivacyStrippingService {
	return &PrivacyStrippingService{
		pii:             NewPIIService(),
		privateTagRegex: regexp.MustCompile(`(?s)<private>.*?</private>`),
		envVarRegex:     regexp.MustCompile(`(?i)(?:export\s+)?(?:API_KEY|SECRET|TOKEN|PASSWORD|PRIVATE_KEY|AWS_ACCESS|AWS_SECRET|DATABASE_URL|REDIS_URL|MONGO_URI|OPENAI_API_KEY|ANTHROPIC_API_KEY|GITHUB_TOKEN)\s*=\s*\S+`),
		secretPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:sk|pk|rk|ak)[-_][a-zA-Z0-9]{20,}`),                        // generic secret keys (sk-xxx, pk-xxx)
			regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),                                              // GitHub PAT
			regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),                                              // GitHub OAuth
			regexp.MustCompile(`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`),                       // GitHub fine-grained PAT
			regexp.MustCompile(`xoxb-[0-9]+-[0-9]+-[a-zA-Z0-9]+`),                                 // Slack bot token
			regexp.MustCompile(`xoxp-[0-9]+-[0-9]+-[0-9]+-[a-f0-9]+`),                             // Slack user token
			regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`), // full private key blocks
			regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                                                  // AWS access key
		},
	}
}

// StrippingResult holds what was stripped.
type StrippingResult struct {
	OriginalLength int      `json:"originalLength"`
	StrippedLength int      `json:"strippedLength"`
	RemovedTypes   []string `json:"removedTypes,omitempty"`
	ItemsRemoved   int      `json:"itemsRemoved"`
}

// Strip removes all sensitive data from content. Unlike Mask, the data is completely gone.
func (s *PrivacyStrippingService) Strip(content string) (string, *StrippingResult) {
	result := &StrippingResult{
		OriginalLength: len(content),
	}
	text := content
	removedTypes := make(map[string]bool)

	// 1. Remove <private>...</private> tags and their content
	if s.privateTagRegex.MatchString(text) {
		text = s.privateTagRegex.ReplaceAllString(text, "")
		removedTypes["private_tags"] = true
	}

	// 2. Remove environment variable assignments with secrets
	if s.envVarRegex.MatchString(text) {
		text = s.envVarRegex.ReplaceAllString(text, "[REDACTED_ENV]")
		removedTypes["env_vars"] = true
	}

	// 3. Remove known secret patterns
	for _, pat := range s.secretPatterns {
		if pat.MatchString(text) {
			text = pat.ReplaceAllString(text, "[REDACTED]")
			removedTypes["secrets"] = true
		}
	}

	// 4. Strip PII (using existing PII patterns but removing instead of masking)
	for _, p := range piiPatterns {
		if p.Pattern.MatchString(text) {
			text = p.Pattern.ReplaceAllString(text, "")
			removedTypes[string(p.Type)] = true
		}
	}

	// 5. Clean up multiple blank lines left by stripping
	text = cleanupWhitespace(text)

	for t := range removedTypes {
		result.RemovedTypes = append(result.RemovedTypes, t)
	}
	result.ItemsRemoved = len(removedTypes)
	result.StrippedLength = len(text)

	return text, result
}

// StripBeforeStorage strips content before persisting to database.
// Returns stripped content. If nothing was stripped, returns original.
func (s *PrivacyStrippingService) StripBeforeStorage(content string) string {
	stripped, result := s.Strip(content)
	if result.ItemsRemoved == 0 {
		return content
	}
	return stripped
}

// ContainsSensitive checks if content has any sensitive data that should be stripped.
func (s *PrivacyStrippingService) ContainsSensitive(content string) bool {
	if s.privateTagRegex.MatchString(content) {
		return true
	}
	if s.envVarRegex.MatchString(content) {
		return true
	}
	for _, pat := range s.secretPatterns {
		if pat.MatchString(content) {
			return true
		}
	}
	return s.pii.ContainsPII(content)
}

func cleanupWhitespace(text string) string {
	// Replace 3+ newlines with 2
	multiNewline := regexp.MustCompile(`\n{3,}`)
	text = multiNewline.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
