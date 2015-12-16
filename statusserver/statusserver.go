package statusserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/JeremyOT/docker-status/internal/github.com/JeremyOT/etcdmon/etcd"
	"github.com/JeremyOT/docker-status/internal/github.com/fsouza/go-dockerclient"
)

const DockerSocket = "unix:///var/run/docker.sock"

type ContainerResponse struct {
	Error      string                 `json:"error,omitempty"`
	Containers []docker.APIContainers `json:"containers,omitempty"`
	Host       string                 `json:"host,omitempty"`
	Port       int                    `json:"port,omitempty"`
}

type ClusterContainerResponse struct {
	Hosts map[string]ContainerResponse `json:"hosts"`
}

type StatusServer struct {
	quit           chan struct{}
	wait           chan struct{}
	connectedAddr  net.Addr
	net            string
	laddr          string
	dockerClient   *docker.Client
	registryConfig etcd.Config
	showSize       bool
}

func New(laddr string, registryConfig etcd.Config) *StatusServer {
	return &StatusServer{
		quit:           make(chan struct{}),
		wait:           make(chan struct{}),
		net:            "tcp",
		laddr:          laddr,
		registryConfig: registryConfig,
	}
}

func (s *StatusServer) ConnectedAddr() string {
	if s.connectedAddr == nil {
		return ""
	}
	return s.connectedAddr.String()
}

func (s *StatusServer) Start() (err error) {
	s.wait = make(chan struct{})
	s.quit = make(chan struct{})
	s.dockerClient, err = docker.NewClient(DockerSocket)
	if err != nil {
		return
	}
	listener, err := net.Listen(s.net, s.laddr)
	if err != nil {
		return
	}
	s.connectedAddr = listener.Addr()
	go s.run(listener)
	return
}

func (s *StatusServer) Stop() {
	if s.quit != nil {
		s.Quit()
		s.Wait()
	}
}

func (s *StatusServer) Quit() {
	close(s.quit)
}

func (s *StatusServer) Wait() {
	<-s.wait
}

func (s *StatusServer) writeError(writer http.ResponseWriter, status int, err error) {
	if status == 0 {
		status = 500
	}
	writer.WriteHeader(status)
	json.NewEncoder(writer).Encode(struct {
		Error string `json:"error"`
	}{Error: err.Error()})
}

func (s *StatusServer) handleLocalStatusRequest(writer http.ResponseWriter, request *http.Request) {
	requestStart := time.Now()
	writer.Header().Add("Content-Type", "application/json")
	containers, err := s.dockerClient.ListContainers(docker.ListContainersOptions{Size: s.showSize})
	if err != nil {
		s.writeError(writer, 500, err)
		return
	}
	for _, c := range containers {
		c.Ports = nil
	}
	json.NewEncoder(writer).Encode(ContainerResponse{Containers: containers})
	log.Println("GET /status/local in", time.Now().Sub(requestStart))
}

func (s *StatusServer) handleClusterStatusRequest(writer http.ResponseWriter, request *http.Request) {
	requestStart := time.Now()
	writer.Header().Add("Content-Type", "application/json")
	config := s.registryConfig
	config.KeyPath = path.Dir(config.KeyPath)
	services, err := etcd.ListServices(config)
	if err != nil {
		s.writeError(writer, 500, err)
		return
	}
	containers := map[string]ContainerResponse{}
	for _, service := range services {
		addr := service.Address()
		response := ContainerResponse{Host: service.Host, Port: service.Port}
		resp, err := http.Get(fmt.Sprintf("http://%s/status/local", addr))
		if err != nil {
			response.Error = err.Error()
			containers[addr] = response
			continue
		}
		defer resp.Body.Close()
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			response.Error = err.Error()
		}
		containers[addr] = response
	}
	json.NewEncoder(writer).Encode(ClusterContainerResponse{Hosts: containers})
	log.Println("GET /status in", time.Now().Sub(requestStart))
}

func (s *StatusServer) run(listener net.Listener) {
	defer close(s.wait)
	defer listener.Close()
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/status", s.handleClusterStatusRequest)
	serveMux.HandleFunc("/status/local", s.handleLocalStatusRequest)
	server := &http.Server{Handler: http.HandlerFunc(serveMux.ServeHTTP)}
	go server.Serve(listener)
	<-s.quit
}
