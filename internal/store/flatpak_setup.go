package store

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo"
)

// InstallFlatpak installs the Flatpak package through this machine's native
// package manager and makes it available to the store immediately (the
// registry is re-detected). It does nothing when Flatpak is already present.
// Flathub is registered afterwards (user scope) so the first app install
// needs no second step.
func (s *Store) InstallFlatpak(ctx context.Context, progress *mygo.Channel[InstallEvent]) error {
	emit := func(ev InstallEvent) { _ = progress.Send(ev) }
	emit(InstallEvent{Type: "start", Message: "Checking for Flatpak…", Percent: -1})

	if hasBinary("flatpak") {
		emit(InstallEvent{Type: "done", Message: "Flatpak is already installed.", Percent: 100})
		return nil
	}
	if s.distro.Family == "" {
		err := fmt.Errorf("couldn't detect your package manager; install Flatpak manually — see https://flatpak.org/setup")
		emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
		return err
	}

	args := []string{"env"}
	switch s.distro.Family {
	case "apt":
		args = append(args, "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", "flatpak")
	case "dnf":
		args = append(args, "dnf", "install", "-y", "flatpak")
	case "pacman":
		args = append(args, "pacman", "-S", "--needed", "--noconfirm", "flatpak")
	case "zypper":
		args = append(args, "zypper", "--non-interactive", "install", "flatpak")
	case "apk":
		args = append(args, "apk", "add", "flatpak")
	default:
		err := fmt.Errorf("don't know how to install Flatpak on this distro; see https://flatpak.org/setup")
		emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
		return err
	}
	if !isRoot() {
		args = append([]string{"pkexec"}, args...)
	}

	emit(InstallEvent{Type: "start", Message: "Installing Flatpak (this may ask for your password)…", Percent: -1})
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	if err := runLines(ctx, emit, args[0], args[1:]...); err != nil {
		emit(InstallEvent{Type: "error", Message: err.Error(), Percent: -1})
		return fmt.Errorf("could not install flatpak: %w", err)
	}

	s.refreshRegistry()
	if err := EnsureFlathub(ctx); err != nil {
		// Flatpak is there; the app will still be usable — Flathub just stays manual.
		emit(InstallEvent{Type: "line", Line: "Flathub could not be added automatically: " + err.Error(), Percent: -1})
	} else {
		emit(InstallEvent{Type: "line", Line: "Flathub remote added (user scope)", Percent: -1})
	}
	emit(InstallEvent{Type: "done", Message: "Flatpak installed — you can install apps now.", Percent: 100})
	return nil
}
