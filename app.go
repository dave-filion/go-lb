package main

import (
	"net/http"
	"log"
	"time"
	"io"
)

var c *http.Client

var activePorts map[string]bool
var serverList []string
var serverPorts = []string{"8888", "8889", "8890"}
var TotalRequests = 0
var ServerOffset = 0


type ProxyHandler struct {
}

func (p ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	TotalRequests += 1
	log.Println("incoming request", TotalRequests)

	tries := 0
	for !activePorts[serverList[ServerOffset]] {
		log.Println("server", serverList[ServerOffset], "is down, trying another")
		ServerOffset += 1
		if ServerOffset >= len(serverList) {
			ServerOffset = 0
		}
		tries += 1
		if tries == len(serverList) {
			log.Fatal("Couldnt find any up servers, quitting")
		}
	}
	server := serverList[ServerOffset]

	// increment offset for next round robin
	ServerOffset += 1
	if ServerOffset >= len(serverList) {
		ServerOffset = 0 // wrap around to beggining
	}
	url := "http://localhost:" + server + r.URL.String()
	log.Println("sending request to", url)
	newreq, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		log.Println("error making req", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := c.Do(newreq)
	if err != nil {
		log.Println("error getting resp", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	io.Copy(w, resp.Body)
}

func pingServer(port string, up chan ServerStatus) {
	url := "http://localhost:" + port + "/ping"
	_, err := c.Get(url)
	if err != nil {
		up <- ServerStatus{false, port}
	} else {
		up <- ServerStatus{true, port}
	}
}

func updateServer(activePorts map[string]bool, port string, status bool) {
	activePorts[port] = status
}

type ServerStatus struct {
	status bool
	port   string
}

func init() {
	// initialize all servers to true
	activePorts = map[string]bool{}
	for _, v := range serverPorts {
		activePorts[v] = true
	}
	// make list of servers for convience
	serverList = []string{}
	for k, _ := range activePorts {
		serverList = append(serverList, k)
	}
	log.Println("server list ->", serverList)

}

func main() {
	c = http.DefaultClient
	// setup pingers
	tick := time.NewTicker(5 * time.Second)
	up := make(chan ServerStatus)

	go func() {
		for {
			select {
			case <-tick.C:
				for k, _ := range activePorts {
					go pingServer(k, up)
				}
			}

		}
	}()

	go func() {
		for {
			select {
			case serverStatus := <-up:
				updateServer(activePorts, serverStatus.port, serverStatus.status)
			}
		}
	}()

	p := ProxyHandler{}
	log.Fatal(http.ListenAndServe(":8080", p))
}
