package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nadoo/glider/dns"
	"github.com/nadoo/glider/ipset"
	"github.com/nadoo/glider/pkg/log"
	"github.com/nadoo/glider/proxy"
	"github.com/nadoo/glider/rule"
	"github.com/nadoo/glider/service"
)

var (
	version = "0.17.0"
	config  = parseConfig()
)

func main() {
	// Check if multi-listener mode is enabled
	if config.UseMultiListenerMode {
		log.F("[main] Multi-listener mode enabled")
		runMultiListenerMode()
	} else {
		log.F("[main] Traditional mode (all listeners share forwarders)")
		runTraditionalMode()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

// runTraditionalMode runs glider in traditional mode (all listeners share forwarders)
func runTraditionalMode() {
	// global rule proxy
	pxy := rule.NewProxy(config.Forwards, &config.Strategy, config.rules)

	// setup API manager for API strategy mode
	if config.ServerPort != "" {
		// 设置全局API管理器
		rule.SetAPIManager(GetAPIManager())

		// 初始化 SXX API（如果配置了 sxxhost）
		if err := InitSXXAPI(config.SXXHost, config.SXXKey); err != nil {
			// SXX API 初始化失败，记录错误但不影响主程序启动
			log.F("[main] SXX API initialization failed: %v", err)
			log.F("[main] SXX Proxy features will be disabled")
		}

		// 启动API服务器
		StartAPIServer(config.ServerPort)
		log.F("[main] API server enabled on port %s", config.ServerPort)
	}

	// ipset manager
	ipsetM, _ := ipset.NewManager(config.rules)

	// check and setup dns server
	if config.DNS != "" {
		d, err := dns.NewServer(config.DNS, pxy, &config.DNSConfig)
		if err != nil {
			log.Fatal(err)
		}

		// rules
		for _, r := range config.rules {
			if len(r.DNSServers) > 0 {
				for _, domain := range r.Domain {
					d.SetServers(domain, r.DNSServers)
				}
			}
		}

		// add a handler to update proxy rules when a domain resolved
		d.AddHandler(pxy.AddDomainIP)
		if ipsetM != nil {
			d.AddHandler(ipsetM.AddDomainIP)
		}

		d.Start()

		// custom resolver
		net.DefaultResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: time.Second * 3}
				return d.DialContext(ctx, "udp", config.DNS)
			},
		}
	}

	for _, r := range config.rules {
		r.IP, r.CIDR, r.Domain = nil, nil, nil
	}

	// enable checkers
	pxy.Check()

	// run proxy servers
	for _, listen := range config.Listens {
		local, err := proxy.ServerFromURL(listen, pxy)
		if err != nil {
			log.Fatal(err)
		}
		go local.ListenAndServe()
	}

	// run services
	for _, s := range config.Services {
		service, err := service.New(s)
		if err != nil {
			log.Fatal(err)
		}
		go service.Run()
	}

	// setup proxy list for API manager if API server is enabled
	if config.ServerPort != "" {
		setupAPIProxyList(pxy)
	}
}

// runMultiListenerMode runs glider in multi-listener mode (each listener has dedicated forwarders)
func runMultiListenerMode() {
	if len(config.ListenerGroups) == 0 {
		log.Fatal("[main] Multi-listener mode enabled but no listener groups configured. Please add [listener-N] sections in config file.")
	}

	log.F("[main] Starting %d listener group(s)", len(config.ListenerGroups))

	// setup API manager for API strategy mode
	if config.ServerPort != "" {
		// 设置全局API管理器
		rule.SetAPIManager(GetAPIManager())

		// 初始化 SXX API（如果配置了 sxxhost）
		if err := InitSXXAPI(config.SXXHost, config.SXXKey); err != nil {
			// SXX API 初始化失败，记录错误但不影响主程序启动
			log.F("[main] SXX API initialization failed: %v", err)
			log.F("[main] SXX Proxy features will be disabled")
		}

		// 启动API服务器
		StartAPIServer(config.ServerPort)
		log.F("[main] API server enabled on port %s", config.ServerPort)
	}

	// Create a proxy for DNS (use first group's forwarders or empty)
	var dnsProxy *rule.Proxy
	if len(config.ListenerGroups) > 0 && len(config.ListenerGroups[0].Forwards) > 0 {
		dnsProxy = rule.NewProxy(config.ListenerGroups[0].Forwards, &config.ListenerGroups[0].Strategy, nil)
	} else {
		// Use direct connection for DNS if no forwarders
		dnsProxy = rule.NewProxy([]string{}, &config.Strategy, nil)
	}

	// check and setup dns server
	if config.DNS != "" {
		d, err := dns.NewServer(config.DNS, dnsProxy, &config.DNSConfig)
		if err != nil {
			log.Fatal(err)
		}

		d.Start()

		// custom resolver
		net.DefaultResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: time.Second * 3}
				return d.DialContext(ctx, "udp", config.DNS)
			},
		}
	}

	// Create and run each listener group with its dedicated forwarders
	for _, group := range config.ListenerGroups {
		if group.Listen == "" {
			log.F("[main] Skipping listener group %s: no listen address configured", group.Name)
			continue
		}

		log.F("[main] Starting listener group: %s, listen=%s, forwards=%d", 
			group.Name, group.Listen, len(group.Forwards))

		// Create dedicated proxy for this listener group
		groupProxy := rule.NewProxy(group.Forwards, &group.Strategy, nil)
		
		// Enable checkers for this group
		groupProxy.Check()

		// Create and start listener
		local, err := proxy.ServerFromURL(group.Listen, groupProxy)
		if err != nil {
			log.F("[main] Failed to create listener for group %s: %v", group.Name, err)
			continue
		}

		// Set IP whitelist if specified
		if group.IPAllow != "" {
			if setter, ok := local.(interface{ SetIPAllow(string) }); ok {
				setter.SetIPAllow(group.IPAllow)
				log.F("[main] IP whitelist enabled for %s: %s", group.Name, group.IPAllow)
			}
		}

		go local.ListenAndServe()

		log.F("[main] Listener group %s started successfully", group.Name)
	}

	// run services
	for _, s := range config.Services {
		service, err := service.New(s)
		if err != nil {
			log.Fatal(err)
		}
		go service.Run()
	}

	// setup proxy list for API manager if API server is enabled
	// In multi-listener mode, use the first group's proxy
	if config.ServerPort != "" && len(config.ListenerGroups) > 0 {
		firstGroupProxy := rule.NewProxy(config.ListenerGroups[0].Forwards, &config.ListenerGroups[0].Strategy, nil)
		setupAPIProxyList(firstGroupProxy)
	}
}

// setupAPIProxyList 设置API管理器的代理列表
func setupAPIProxyList(pxy *rule.Proxy) {
	// 获取主转发器组的代理列表
	if mainGroup := pxy.GetMainGroup(); mainGroup != nil {
		proxies := mainGroup.GetForwarders()
		if len(proxies) > 0 {
			GetAPIManager().SetProxyList(proxies)
			log.F("[main] API manager initialized with %d proxies", len(proxies))
		} else {
			log.F("[main] Warning: No proxies available for API manager")
		}
	}
}
