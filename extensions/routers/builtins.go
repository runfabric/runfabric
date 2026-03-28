package routers

import (
	azurerouter "github.com/runfabric/runfabric/extensions/routers/azuretrafficmanager"
	cloudflarerouter "github.com/runfabric/runfabric/extensions/routers/cloudflare"
	ns1router "github.com/runfabric/runfabric/extensions/routers/ns1"
	route53router "github.com/runfabric/runfabric/extensions/routers/route53"
	sdkrouter "github.com/runfabric/runfabric/plugin-sdk/go/router"
)

func BuiltinRouterManifests() []sdkrouter.PluginMeta {
	return []sdkrouter.PluginMeta{
		cloudflarerouter.RouterMeta(),
		route53router.RouterMeta(),
		ns1router.RouterMeta(),
		azurerouter.RouterMeta(),
	}
}

func NewBuiltinRegistry() *Registry {
	reg := NewRegistry()
	_ = reg.Register(cloudflarerouter.NewRouter())
	_ = reg.Register(route53router.NewRouter())
	_ = reg.Register(ns1router.NewRouter())
	_ = reg.Register(azurerouter.NewRouter())
	return reg
}
