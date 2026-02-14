package diff

import "testing"

func TestFilterLockFiles(t *testing.T) {
	files := []FileDiff{
		{Filename: "package-lock.json", Raw: "some diff", Additions: 100, Deletions: 50},
		{Filename: "src/main.go", Raw: "+func main() {}", Additions: 1},
	}

	result := Filter(files, nil)

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file after filter, got %d", len(result.Files))
	}
	if result.Files[0].Filename != "src/main.go" {
		t.Errorf("expected src/main.go to survive filter, got %s", result.Files[0].Filename)
	}
	if len(result.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(result.Notes))
	}
	if result.Notes[0] != "[lock file updated: package-lock.json]" {
		t.Errorf("unexpected note: %s", result.Notes[0])
	}
}

func TestFilterBinaryFiles(t *testing.T) {
	files := []FileDiff{
		{Filename: "image.png", IsBinary: true},
		{Filename: "src/app.go", Raw: "+code", Additions: 5},
	}

	result := Filter(files, nil)

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file after filter, got %d", len(result.Files))
	}
	if len(result.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(result.Notes))
	}
}

func TestFilterGeneratedFiles(t *testing.T) {
	files := []FileDiff{
		{Filename: "api.pb.go", Raw: "generated code", Additions: 500},
		{Filename: "app.min.js", Raw: "minified code", Additions: 1},
		{Filename: "src/real.go", Raw: "+real code", Additions: 10},
	}

	result := Filter(files, nil)

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file after filter, got %d", len(result.Files))
	}
	if len(result.Notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(result.Notes))
	}
}

func TestFilterLargeHunks(t *testing.T) {
	files := []FileDiff{
		{Filename: "migration.sql", Raw: "big migration", Additions: 150, Deletions: 100},
		{Filename: "src/app.go", Raw: "+small change", Additions: 5},
	}

	result := Filter(files, nil)

	// migration.sql has 250 total changes (>200), should be collapsed
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file after filter, got %d", len(result.Files))
	}
	if result.Files[0].Filename != "src/app.go" {
		t.Errorf("expected src/app.go to survive, got %s", result.Files[0].Filename)
	}
}

func TestFilterExcludePatterns(t *testing.T) {
	files := []FileDiff{
		{Filename: "icon.svg", Raw: "svg data", Additions: 1},
		{Filename: "src/app.go", Raw: "+code", Additions: 5},
	}

	result := Filter(files, []string{"*.svg"})

	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file after filter, got %d", len(result.Files))
	}
}
