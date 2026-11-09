package updates

import (
	"context"
	"errors"
	"runtime"

	"callbell/internal/buildinfo"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
)

// Configure is called once before checks begin. No global HTTP defaults or IM
// transport state is changed. The release packager embeds the trusted public key.
func Configure() Checker {
	source := NewSource(buildinfo.Repository, Target(runtime.GOOS, runtime.GOARCH), netproxy.Transport())
	mygo.Updater.SetSource(source)
	mygo.Updater.SetPrereleases(false)
	return func(ctx context.Context) (*Release, error) {
		up, err := mygo.Updater.Check(ctx)
		if errors.Is(err, mygo.ErrUpdatesDisabled) {
			return source.CheckReadOnly(ctx, buildinfo.Version)
		}
		if err != nil || up == nil {
			return nil, err
		}
		return &Release{Version: up.Version, Install: up.Install}, nil
	}
}
