package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/JeremyOT/docker-status/internal/github.com/JeremyOT/address/lookup"
	"github.com/JeremyOT/docker-status/internal/github.com/JeremyOT/etcdmon/etcd"
	"github.com/JeremyOT/docker-status/statusserver"
)

func monitorSignal(registry *etcd.Registry, sigChan <-chan os.Signal) {
	for sig := range sigChan {
		log.Println("Received signal", sig, "exiting immediately")
		fmt.Println("Received signal", sig, "exiting immediately")
		registry.Stop()
	}
}

func main() {
	config := struct {
		etcdURL string
		etcdKey string
		port    int
	}{}
	flag.StringVar(&config.etcdURL, "etcd", "", "The url of the etcd instance to connect to.")
	flag.StringVar(&config.etcdKey, "key", "services/docker-status/%H-%P", "The key to post to.")
	flag.IntVar(&config.port, "port", 0, "The port to listen on, or 0 to pick one.")
	flag.Parse()

	if config.etcdURL == "" {
		panic("Missing required argument -etcd")
	}
	if config.etcdKey == "" {
		panic("Missing required argument -key")
	}
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	registryConfig := etcd.Config{
		EtcdHost:       config.etcdURL,
		Key:            config.etcdKey,
		TTL:            5 * time.Minute,
		UpdateInterval: 1 * time.Minute,
		Port:           config.port,
	}
	if registryConfig.Port == 0 {
		port, err := lookup.FindOpenTCPPort("", true)
		if err != nil {
			panic(err)
		}
		registryConfig.Port = port
	}
	registryConfig, err := registryConfig.Populate()
	fmt.Printf("%#v\n", registryConfig)
	if err != nil {
		panic(err)
	}
	registry := etcd.NewRegistry(registryConfig)
	go monitorSignal(registry, sigChan)
	registry.Start()
	server := statusserver.New(net.JoinHostPort("", strconv.Itoa(registryConfig.Port)), registryConfig)
	if err = server.Start(); err != nil {
		panic(err)
	}
	log.Println("Listening on:", server.ConnectedAddr())
	defer server.Stop()
	registry.Wait()
}
