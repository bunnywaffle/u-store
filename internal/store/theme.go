package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// Theme modes the app understands. "system" follows the desktop (dark when
// there is no hint — see the CSS, where dark is the fallback palette).
const (
	ThemeDark   = "dark"
	ThemeLight  = "light"
	ThemeSystem = "system"
)

// AppTheme applies a theme mode to mygo.Theme; unknown values fall back to system.
func AppTheme(mode string) {
	switch mode {
	case ThemeDark:
		mygo.Theme.SetSource(mygo.ThemeDark)
	case ThemeLight:
		mygo.Theme.SetSource(mygo.ThemeLight)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

func themeFilePath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "theme"), nil
}

// LoadTheme reads the persisted theme mode; "system" when unset or unreadable.
func LoadTheme() string {
	path, err := themeFilePath()
	if err != nil {
		return ThemeSystem
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ThemeSystem
	}
	mode := strings.TrimSpace(string(data))
	switch mode {
	case ThemeDark, ThemeLight, ThemeSystem:
		return mode
	default:
		return ThemeSystem
	}
}

// saveTheme writes the persisted theme mode.
func saveTheme(mode string) error {
	path, err := themeFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(mode), 0o644)
}

// SetTheme persists and applies the mode. Bound to the frontend.
func (s *Store) SetTheme(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case ThemeDark, ThemeLight, ThemeSystem:
	default:
		return fmt.Errorf("unknown theme %q (want dark, light or system)", mode)
	}
	if err := saveTheme(mode); err != nil {
		return err
	}
	s.theme = mode
	AppTheme(mode)
	return nil
}

// ApplyThemeMode records the mode without persisting or applying it — used by
// main at startup, after LoadTheme + AppTheme have done both.
func ApplyThemeMode(s *Store, mode string) {
	switch mode {
	case ThemeDark, ThemeLight, ThemeSystem:
		s.theme = mode
	default:
		s.theme = ThemeSystem
	}
}
