package diff

import "testing"

const sampleDiff = `diff --git a/main.go b/main.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/main.go
@@ -0,0 +1,10 @@
+package main
+
+import "fmt"
+
+func main() {
+	fmt.Println("hello")
+}
diff --git a/README.md b/README.md
index 1234567..abcdefg 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,5 @@
 # Project
+
+Added a description.
-Old line
+New line
`

func TestParse(t *testing.T) {
	files := Parse(sampleDiff)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// First file: main.go (new file)
	if files[0].Filename != "main.go" {
		t.Errorf("expected filename main.go, got %s", files[0].Filename)
	}
	if !files[0].IsNew {
		t.Error("expected main.go to be marked as new")
	}
	if files[0].Additions != 7 {
		t.Errorf("expected 7 additions in main.go, got %d", files[0].Additions)
	}

	// Second file: README.md
	if files[1].Filename != "README.md" {
		t.Errorf("expected filename README.md, got %s", files[1].Filename)
	}
	if files[1].IsNew {
		t.Error("README.md should not be marked as new")
	}
	if files[1].Additions != 3 {
		t.Errorf("expected 3 additions in README.md, got %d", files[1].Additions)
	}
	if files[1].Deletions != 1 {
		t.Errorf("expected 1 deletion in README.md, got %d", files[1].Deletions)
	}
}

func TestParseEmpty(t *testing.T) {
	files := Parse("")
	if files != nil {
		t.Errorf("expected nil for empty diff, got %v", files)
	}
}

func TestParseRename(t *testing.T) {
	diff := `diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
`
	files := Parse(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Filename != "new.go" {
		t.Errorf("expected filename new.go, got %s", files[0].Filename)
	}
	if files[0].OldFilename != "old.go" {
		t.Errorf("expected old filename old.go, got %s", files[0].OldFilename)
	}
}
