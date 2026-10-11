package shellmanager

import (
	"context"
	"maps"
	"slices"
	"strings"
)

// aptBackend tools are installed by the preflight, in one privileged
// apt-get call together with every tool's system_packages; Install and
// Update have nothing left to do.
type aptBackend struct{}

func (aptBackend) Install(context.Context, Env, Tool) error { return nil }
func (aptBackend) Update(context.Context, Env, Tool) error  { return nil }

// aptPackages lists every apt package the tools need, sorted and unique.
func aptPackages(tools []Tool) []string {
	set := map[string]bool{}
	for _, t := range tools {
		for _, p := range t.SystemPackages {
			set[p] = true
		}
		if t.Install.Backend == "apt" {
			set[t.Install.Package] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// missingAptPackages returns the packages dpkg does not report installed.
func missingAptPackages(ctx context.Context, sys System, pkgs []string) []string {
	var missing []string
	for _, p := range pkgs {
		out, err := sys.Run(ctx, Cmd{Name: "dpkg-query", Args: []string{"-W", "-f=${Status}", p}})
		if err != nil || !strings.Contains(string(out), "install ok installed") {
			missing = append(missing, p)
		}
	}
	return missing
}
