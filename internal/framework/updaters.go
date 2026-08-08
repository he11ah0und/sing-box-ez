package framework

import (
	"fmt"

	"github.com/he11ah0und/projectspec"
	"sing-box-ez/internal/framework/fs"
	"sing-box-ez/internal/framework/updater"
)

// buildUpdatersFromSpec constructs updater managers from the parsed project
// spec. A spec error is a build-time developer mistake, so every problem is
// reported in the returned error.
func (a *App) buildUpdatersFromSpec(spec *projectspec.Spec, loadInstallScript func(string) []byte) ([]*updater.Manager, error) {

	managers := make([]*updater.Manager, 0, len(spec.Updaters))
	for _, u := range spec.Updaters {
		mgr := updater.NewManager(a.Logger.Root, u.Name)
		switch u.Source.Backend {
		case "github":
			if u.Source.BaseURL != "" {
				mgr.Source = updater.NewGitHubEnterpriseBackend(mgr.Log, u.Source.BaseURL, u.Source.Owner, u.Source.Repo)
			} else {
				mgr.Source = updater.NewGitHubBackend(mgr.Log, u.Source.Owner, u.Source.Repo)
			}
		default:
			return nil, fmt.Errorf("updaters[%q]: unsupported source backend %q", u.Name, u.Source.Backend)
		}
		if len(u.Source.AssetTags) > 0 {
			mgr.AssetCriteria = updater.AssetCriteria{Tags: u.Source.AssetTags}
		}
		switch u.Apply.Backend {
		case "self":
			mgr.Apply = updater.NewSelfUpdateApply(mgr.Log, fs.NewOSWithLog(u.Apply.BaseDir, mgr.Log.Allocate("fs")))
		case "files":
			apply := updater.NewFilesUpdateApply(mgr.Log, fs.NewOSWithLog(u.Apply.BaseDir, mgr.Log.Allocate("fs")))
			apply.BaseDir = u.Apply.BaseDir
			if u.Apply.InstallScript != "" {
				if loadInstallScript == nil {
					return nil, fmt.Errorf("updaters[%q]: install_script requires LoadInstallScript", u.Name)
				}
				apply.InstallScript = loadInstallScript(u.Apply.InstallScript)
			}
			mgr.Apply = apply
		default:
			return nil, fmt.Errorf("updaters[%q]: unsupported apply backend %q", u.Name, u.Apply.Backend)
		}
		managers = append(managers, mgr)
	}
	return managers, nil
}
