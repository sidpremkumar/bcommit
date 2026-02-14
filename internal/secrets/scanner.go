package secrets

import (
	"fmt"
	"regexp"
	"strings"
)

// Finding represents a detected secret in the diff.
type Finding struct {
	Filename string
	Line     string
	Rule     string
}

type rule struct {
	name    string
	pattern *regexp.Regexp
}

var rules = []rule{
	// AWS
	{"AWS Access Key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"AWS Secret Key", regexp.MustCompile(`(?i)(aws_secret_access_key|aws_secret)\s*[:=]\s*[A-Za-z0-9/+=]{40}`)},

	// Generic API keys and tokens
	{"Generic API Key", regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*["']?[A-Za-z0-9_\-]{20,}["']?`)},
	{"Generic Secret", regexp.MustCompile(`(?i)(secret|secret[_-]?key)\s*[:=]\s*["']?[A-Za-z0-9_\-]{20,}["']?`)},
	{"Generic Token", regexp.MustCompile(`(?i)(access[_-]?token|auth[_-]?token)\s*[:=]\s*["']?[A-Za-z0-9_\-\.]{20,}["']?`)},

	// Private keys
	{"Private Key", regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`)},

	// GitHub
	{"GitHub Token (ghp)", regexp.MustCompile(`ghp_[A-Za-z0-9_]{36,}`)},
	{"GitHub Token (gho)", regexp.MustCompile(`gho_[A-Za-z0-9_]{36,}`)},
	{"GitHub Token (ghu)", regexp.MustCompile(`ghu_[A-Za-z0-9_]{36,}`)},
	{"GitHub Token (ghs)", regexp.MustCompile(`ghs_[A-Za-z0-9_]{36,}`)},
	{"GitHub Token (ghr)", regexp.MustCompile(`ghr_[A-Za-z0-9_]{36,}`)},
	{"GitHub PAT (classic)", regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`)},

	// Slack
	{"Slack Token", regexp.MustCompile(`xox[bpors]-[0-9]{10,}-[A-Za-z0-9-]+`)},
	{"Slack Webhook", regexp.MustCompile(`https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+`)},

	// Google
	{"Google API Key", regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},
	{"Google OAuth Secret", regexp.MustCompile(`(?i)client_secret.*\.apps\.googleusercontent\.com`)},

	// Stripe
	{"Stripe Secret Key", regexp.MustCompile(`sk_live_[0-9a-zA-Z]{24,}`)},
	{"Stripe Publishable Key", regexp.MustCompile(`pk_live_[0-9a-zA-Z]{24,}`)},

	// Database connection strings
	{"Database URL", regexp.MustCompile(`(?i)(postgres|mysql|mongodb(\+srv)?|redis)://[^\s"']{10,}`)},

	// Passwords in config
	{"Password Assignment", regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["'][^"']{8,}["']`)},

	// JWT
	{"JWT Token", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+`)},

	// Heroku
	{"Heroku API Key", regexp.MustCompile(`(?i)heroku.*[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)},

	// SendGrid
	{"SendGrid API Key", regexp.MustCompile(`SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`)},

	// Twilio
	{"Twilio API Key", regexp.MustCompile(`SK[0-9a-fA-F]{32}`)},

	// npm
	{"npm Token", regexp.MustCompile(`npm_[A-Za-z0-9]{36}`)},

	// Generic high-entropy hex strings that look like secrets
	{"Hex Secret (32+ chars)", regexp.MustCompile(`(?i)(token|key|secret|password|credential)\s*[:=]\s*["']?[0-9a-f]{32,}["']?`)},
}

// ScanDiff scans a raw unified diff for secrets in added lines only.
func ScanDiff(rawDiff string) []Finding {
	var findings []Finding

	lines := strings.Split(rawDiff, "\n")
	currentFile := ""

	for _, line := range lines {
		// Track current file
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				currentFile = parts[1]
			}
			continue
		}

		// Only scan added lines (lines starting with +, but not +++ header)
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}

		content := line[1:] // strip the leading +

		for _, r := range rules {
			if r.pattern.MatchString(content) {
				findings = append(findings, Finding{
					Filename: currentFile,
					Line:     truncate(strings.TrimSpace(content), 80),
					Rule:     r.name,
				})
				break // one finding per line is enough
			}
		}
	}

	return deduplicate(findings)
}

// FormatWarnings formats findings into a user-facing warning string.
func FormatWarnings(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n⚠ Potential secrets detected (%d finding(s)):\n\n", len(findings)))

	for _, f := range findings {
		b.WriteString(fmt.Sprintf("  %-25s %s\n", f.Rule+":", f.Filename))
		b.WriteString(fmt.Sprintf("  %s\n\n", maskSecret(f.Line)))
	}

	b.WriteString("  Review carefully before committing!\n")
	return b.String()
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// maskSecret partially masks the detected secret value for display.
func maskSecret(line string) string {
	// Show first 20 and last 5 chars, mask the middle
	if len(line) <= 30 {
		return line
	}
	return line[:20] + "****" + line[len(line)-5:]
}

// deduplicate removes duplicate findings (same file + same rule).
func deduplicate(findings []Finding) []Finding {
	seen := make(map[string]bool)
	var result []Finding
	for _, f := range findings {
		key := f.Filename + "|" + f.Rule + "|" + f.Line
		if !seen[key] {
			seen[key] = true
			result = append(result, f)
		}
	}
	return result
}
