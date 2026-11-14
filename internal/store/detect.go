package store

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// etcOSRelease is overridable for tests.
var etcOSRelease = "/etc/os-release"

// Distro describes the running distribution, parsed from os-release.
type Distro struct {
	ID        string
	IDLike    []string
	Name      string
	Pretty    string
	Arch      string
	Family    string // apt | dnf | pacman | zypper | apk | ""
}

// DetectDistro parses os-release and maps it to a package manager family.
func DetectDistro() Distro {
	d := Distro{Arch: runtime.GOARCH, Family: ""}
	fields := map[string]string{}
	if f, err := os.Open(etcOSRelease); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			fields[key] = strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	d.ID = strings.ToLower(fields["ID"])
	d.Name = fields["NAME"]
	d.Pretty = fields["PRETTY_NAME"]
	if d.Pretty == "" {
		d.Pretty = d.Name
	}
	if d.Name == "" {
		d.Name = d.Pretty
	}
	for _, like := range strings.Fields(fields["ID_LIKE"]) {
		d.IDLike = append(d.IDLike, strings.ToLower(like))
	}
	d.Family = familyOf(d.ID, d.IDLike)
	return d
}

// familyOf maps an os-release ID (and its ID_LIKE chain) to the native
// package manager this family uses.
func familyOf(id string, idLike []string) string {
	has := func(names ...string) bool {
		for _, n := range names {
			if n == id {
				return true
			}
			for _, like := range idLike {
				if like == n {
					return true
				}
			}
		}
		return false
	}
	switch {
	case has("debian", "ubuntu", "linuxmint", "pop", "neon", "zorin", "kali", "mx", "elementary", "raspbian"):
		return "apt"
	case has("fedora", "rhel", "centos", "rocky", "alma", "ol", "amzn"):
		return "dnf"
	case has("arch", "manjaro", "endeavouros", "artix", "garuda", "cachyos"):
		return "pacman"
	case has("opensuse", "opensuse-leap", "opensuse-tumbleweed", "sled", "sles", "suse"):
		return "zypper"
	case has("alpine"):
		return "apk"
	default:
		return ""
	}
}

// hasBinary reports whether an executable exists on PATH.
func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
