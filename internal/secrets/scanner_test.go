package secrets

import (
	"testing"
)

func TestScanDiff_AWSAccessKey(t *testing.T) {
	diff := `diff --git a/config.go b/config.go
--- a/config.go
+++ b/config.go
@@ -1,3 +1,4 @@
 package main
+var awsKey = "AKIAIOSFODNN7EXAMPLE"
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect AWS access key")
	}
	if findings[0].Rule != "AWS Access Key" {
		t.Errorf("expected rule 'AWS Access Key', got %q", findings[0].Rule)
	}
	if findings[0].Filename != "config.go" {
		t.Errorf("expected filename 'config.go', got %q", findings[0].Filename)
	}
}

func TestScanDiff_GitHubToken(t *testing.T) {
	diff := `diff --git a/.env b/.env
--- a/.env
+++ b/.env
@@ -0,0 +1 @@
+GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect GitHub token")
	}
	found := false
	for _, f := range findings {
		if f.Rule == "GitHub Token (ghp)" || f.Rule == "GitHub PAT (classic)" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected GitHub token rule, got %q", findings[0].Rule)
	}
}

func TestScanDiff_PrivateKey(t *testing.T) {
	diff := `diff --git a/key.pem b/key.pem
+++ b/key.pem
@@ -0,0 +1,3 @@
+-----BEGIN RSA PRIVATE KEY-----
+MIIEpAIBAAKCAQEA...
+-----END RSA PRIVATE KEY-----
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect private key")
	}
	if findings[0].Rule != "Private Key" {
		t.Errorf("expected rule 'Private Key', got %q", findings[0].Rule)
	}
}

func TestScanDiff_GenericAPIKey(t *testing.T) {
	diff := `diff --git a/app.py b/app.py
+++ b/app.py
@@ -1,2 +1,3 @@
 import os
+API_KEY = "sk_test_abcdefghijklmnopqrstuvwxyz1234"
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect generic API key")
	}
}

func TestScanDiff_DatabaseURL(t *testing.T) {
	diff := `diff --git a/.env b/.env
+++ b/.env
@@ -0,0 +1 @@
+DATABASE_URL=postgres://user:password123@db.example.com:5432/mydb
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect database URL")
	}
	if findings[0].Rule != "Database URL" {
		t.Errorf("expected rule 'Database URL', got %q", findings[0].Rule)
	}
}

func TestScanDiff_PasswordAssignment(t *testing.T) {
	diff := `diff --git a/config.yaml b/config.yaml
+++ b/config.yaml
@@ -0,0 +1 @@
+password: "my_super_secret_password_123"
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect password assignment")
	}
	if findings[0].Rule != "Password Assignment" {
		t.Errorf("expected rule 'Password Assignment', got %q", findings[0].Rule)
	}
}

func TestScanDiff_JWT(t *testing.T) {
	diff := `diff --git a/auth.go b/auth.go
+++ b/auth.go
@@ -0,0 +1 @@
+var token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect JWT token")
	}
	if findings[0].Rule != "JWT Token" {
		t.Errorf("expected rule 'JWT Token', got %q", findings[0].Rule)
	}
}

func TestScanDiff_SlackToken(t *testing.T) {
	diff := `diff --git a/bot.py b/bot.py
+++ b/bot.py
@@ -0,0 +1 @@
+SLACK_TOKEN = "xoxb-1234567890-abcdefghijklmnop"
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect Slack token")
	}
	if findings[0].Rule != "Slack Token" {
		t.Errorf("expected rule 'Slack Token', got %q", findings[0].Rule)
	}
}

func TestScanDiff_NoSecrets(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
+func main() { fmt.Println("hello") }
`
	findings := ScanDiff(diff)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestScanDiff_IgnoresRemovedLines(t *testing.T) {
	diff := `diff --git a/.env b/.env
--- a/.env
+++ b/.env
@@ -1,2 +1,1 @@
-AWS_KEY=AKIAIOSFODNN7EXAMPLE
+# key removed
`
	findings := ScanDiff(diff)
	if len(findings) != 0 {
		t.Errorf("expected no findings for removed lines, got %d", len(findings))
	}
}

func TestScanDiff_StripeKey(t *testing.T) {
	diff := `diff --git a/billing.go b/billing.go
+++ b/billing.go
@@ -0,0 +1 @@
+var stripeKey = "sk_live_abcdefghijklmnopqrstuvwxyz"
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect Stripe secret key")
	}
	if findings[0].Rule != "Stripe Secret Key" {
		t.Errorf("expected rule 'Stripe Secret Key', got %q", findings[0].Rule)
	}
}

func TestScanDiff_GoogleAPIKey(t *testing.T) {
	diff := `diff --git a/maps.js b/maps.js
+++ b/maps.js
@@ -0,0 +1 @@
+const key = "AIzaSyA1234567890abcdefghijklmnopqrstuvw"
`
	findings := ScanDiff(diff)
	if len(findings) == 0 {
		t.Fatal("expected to detect Google API key")
	}
	if findings[0].Rule != "Google API Key" {
		t.Errorf("expected rule 'Google API Key', got %q", findings[0].Rule)
	}
}

func TestFormatWarnings_Empty(t *testing.T) {
	result := FormatWarnings(nil)
	if result != "" {
		t.Errorf("expected empty string for no findings, got %q", result)
	}
}

func TestFormatWarnings_NonEmpty(t *testing.T) {
	findings := []Finding{
		{Filename: "config.go", Line: `var key = "AKIAIOSFODNN7EXAMPLE"`, Rule: "AWS Access Key"},
	}
	result := FormatWarnings(findings)
	if result == "" {
		t.Fatal("expected non-empty warning string")
	}
}

func TestMaskSecret(t *testing.T) {
	short := "short"
	if got := maskSecret(short); got != short {
		t.Errorf("short strings should not be masked, got %q", got)
	}

	long := "this is a very long secret value that should be masked"
	masked := maskSecret(long)
	if masked == long {
		t.Error("long strings should be masked")
	}
}
