package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/dotcloud/docker"
	"github.com/shipyard/go-dockerclient"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const VERSION string = "0.0.4"

var (
	dockerURL     string
	shipyardURL   string
	shipyardKey   string
	runInterval   int
	registerAgent bool
	version       bool
	port          int
)

type (
	AgentData struct {
		Key string `json:"key"`
	}

	ContainerData struct {
		Container docker.APIContainers
		Meta      *docker.Container
	}

	Job struct {
		Path string
		Data interface{}
	}
)

func init() {
	flag.StringVar(&dockerURL, "host", "http://127.0.0.1:4243", "Docker URL")
	flag.StringVar(&shipyardURL, "url", "", "Shipyard URL")
	flag.StringVar(&shipyardKey, "key", "", "Shipyard Agent Key")
	flag.IntVar(&runInterval, "interval", 5, "Run interval")
	flag.BoolVar(&registerAgent, "register", false, "Register Agent with Shipyard")
	flag.BoolVar(&version, "version", false, "Shows Agent Version")
	flag.IntVar(&port, "port", 4500, "Agent Listen Port")

	flag.Parse()

	if version {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	if shipyardURL == "" {
		fmt.Println("Error: You must specify a Shipyard URL")
		os.Exit(1)
	}
}

func updater(jobs <-chan *Job, group *sync.WaitGroup) {
	group.Add(1)
	defer group.Done()
	client := &http.Client{}

	for obj := range jobs {
		buf := bytes.NewBuffer(nil)
		if err := json.NewEncoder(buf).Encode(obj.Data); err != nil {
			log.Println(err)
			continue
		}

		req, err := http.NewRequest("POST", path.Join(shipyardURL, obj.Path), buf)
		if err != nil {
			log.Println(err)
			continue
		}

		req.Header.Set("Authorization", fmt.Sprintf("AgentKey:%s", shipyardKey))
		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
			continue
		}
		defer resp.Body.Close()
	}
}

func pushContainers(client *dockerclient.Client, jobs chan *Job) {
	containers, err := client.ListContainers(dockerclient.ListContainersOptions{All: true})
	if err != nil {
		log.Fatal(err)
	}
	data := make([]ContainerData, len(containers))
	for x, c := range containers {
		i, err := client.InspectContainer(c.ID)
		if err != nil {
			log.Fatal(err)
		}
		containerData := ContainerData{Container: c, Meta: i}
		data[x] = containerData
	}

	jobs <- &Job{
		Path: "/agent/containers/",
		Data: data,
	}
}

func pushImages(client *dockerclient.Client, jobs chan *Job) {
	images, err := client.ListImages(false)
	if err != nil {
		log.Fatal(err)
	}

	jobs <- &Job{
		Path: "/agent/images/",
		Data: images,
	}
}

func listen(d time.Duration) {
	var (
		group = &sync.WaitGroup{}
		// create chan with a 2 buffer, we use a 2 buffer to sync the go routines so that
		// no more than two messages are being send to the server at one time
		jobs = make(chan *Job, 2)
	)

	client, err := dockerclient.NewClient(dockerURL)
	if err != nil {
		log.Fatal(err)
	}

	go updater(jobs, group)

	for _ := range time.Tick(d) {
		// TODO: is it ok for 10 of these to be running in parallel or do we need to wait?
		go pushContainers(client, jobs)
		go pushImages(client, jobs)
	}

	// wait for all request to finish processing before returning
	group.Wait()
}

// Registers with Shipyard at the specified URL
func register() {
	log.Printf("Registering at %s\n", shipyardURL)

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	var (
		vals = url.Values{"name": {hostname}, "port": {strconv.Itoa(port)}}
		data AgentData
	)

	resp, err := http.PostForm(path.Join(shipyardURL, "agent", "register"), vals)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Fatal(err)
	}
	log.Println("Agent Key: ", data.Key)
}

func main() {
	if registerAgent {
		register()
		return
	}

	duration, err := time.ParseDuration(fmt.Sprintf("%ds", runInterval))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Shipyard Agetn (%s)\n", shipyardURL)
	u, err := url.Parse(dockerURL)
	if err != nil {
		log.Fatal(err)
	}

	var (
		proxy    = httputil.NewSingleHostReverseProxy(u)
		director = proxy.Director
	)

	proxy.Director = func(req *http.Request) {
		src := strings.Split(req.RemoteAddr, ":")[0]
		log.Printf("Request from %s: %s\n", src, req.URL.Path)
		director(req)
	}

	go listen(duration)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), proxy); err != nil {
		log.Fatal(err)
	}
}
