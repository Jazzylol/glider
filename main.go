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
	// global rule proxy
	pxy := rule.NewProxy(config.Forwards, &config.Strategy, config.rules)
	
	// setup API manager for API strategy mode
	if config.ServerPort != "" {
		// 设置全局API管理器
		rule.SetAPIManager(GetAPIManager())
		
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
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
