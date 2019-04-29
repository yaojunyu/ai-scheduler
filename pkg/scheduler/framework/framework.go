package framework

import (
	"gitlab.aibee.cn/platform/ai-scheduler/pkg/scheduler/internal/cache"
	"sync"
)

// OpenSession init session when open
func OpenSession(cache cache.Cache) *Session {
	s := openSession(cache)

	for _, plugin := range s.Plugins {
		plugin.OnSessionOpen(s)
	}

	return s
}

// CloseSeesion clean session
func CloseSession(s *Session) {
	for _, plugin := range s.Plugins {
		plugin.OnSessionClose(s)
	}

	closeSession(s)
}

var (
	lock sync.Mutex

	plugins = map[string]Plugin{}
	actions = map[string]Action{}

)

func Actions() map[string]Action {
	return actions
}

func Plugins() map[string]Plugin {
	return plugins
}

// RegisterPluginBuilder
func RegisterPlugin(plugin Plugin) {
	lock.Lock()
	defer lock.Unlock()

	plugins[plugin.Name()] = plugin
}

// CleanPluginBuilder
func CleanPlugin(name string) {
	lock.Lock()
	defer lock.Unlock()

	plugins = map[string]Plugin{}
}

// GetPluginBuilder
func GetPlugin(name string) (Plugin, bool) {
	lock.Lock()
	defer lock.Unlock()

	pb, ok := plugins[name]
	return pb, ok
}

// GetPluginBuilder
func RegisterAction(act Action) {
	lock.Lock()
	lock.Unlock()

	actions[act.Name()] = act
}

// GetAction
func GetAction(name string) (Action, bool) {
	lock.Lock()
	defer lock.Unlock()

	act, ok := actions[name]
	return act, ok
}