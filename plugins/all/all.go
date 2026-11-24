package all

import (
	"sub2api-panel/internal/pluginrt"
	_ "sub2api-panel/plugins/patrol"
	_ "sub2api-panel/plugins/probelmfpd"
)

func Manifests() []pluginrt.Manifest {
	return pluginrt.Registered()
}
