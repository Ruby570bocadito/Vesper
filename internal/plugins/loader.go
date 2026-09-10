//go:build linux

package plugins

import (
	"os"
	"path/filepath"
	"plugin"
	"sync"
)

type PluginManager struct {
	mu        sync.RWMutex
	loaded    map[string]*PluginInstance
	pluginDir string
}

type PluginInstance struct {
	Name    string
	Version string
	Path    string
	Symbol  plugin.Symbol
	Loaded  bool
}

func NewPluginManager(dir string) *PluginManager {
	os.MkdirAll(dir, 0755)
	return &PluginManager{loaded: make(map[string]*PluginInstance), pluginDir: dir}
}

func (pm *PluginManager) LoadAll() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	loaded := 0
	soFiles, _ := filepath.Glob(filepath.Join(pm.pluginDir, "*.so"))
	for _, soFile := range soFiles {
		p, err := plugin.Open(soFile)
		if err != nil {
			continue
		}
		pm.loaded[filepath.Base(soFile)] = &PluginInstance{Name: filepath.Base(soFile), Path: soFile, Symbol: p, Loaded: true}
		loaded++
	}
	return loaded
}

func (pm *PluginManager) HotReload() int {
	return pm.LoadAll()
}

func (pm *PluginManager) GetModules() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	mods := make([]string, 0, len(pm.loaded))
	for k := range pm.loaded {
		mods = append(mods, k)
	}
	return mods
}

func (pm *PluginManager) GetCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.loaded)
}
