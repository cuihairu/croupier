package game

import "testing"

func TestSanitizeGameName(t *testing.T) {
	t.Parallel()

	valid := []string{"game1", "game_1", "game-1", "game@1", "GAME_ABC-123"}
	for _, name := range valid {
		name := name
		t.Run("valid_"+name, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizeGameName(name)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != name {
				t.Fatalf("expected %q, got %q", name, got)
			}
		})
	}

	invalid := []string{"", " ", "game 1", "game.1", "game/1", "中文"}
	for _, name := range invalid {
		name := name
		t.Run("invalid_"+name, func(t *testing.T) {
			t.Parallel()
			if _, err := sanitizeGameName(name); err == nil {
				t.Fatalf("expected error for %q", name)
			}
		})
	}
}
