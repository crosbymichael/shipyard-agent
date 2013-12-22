package main

import (
    dockerclient "github.com/fsouza/go-dockerclient"
    "github.com/dotcloud/docker"
    _ "github.com/shipyard/shipyard-go/shipyard"
    "fmt"
    "os"
    "io"
    "flag"
    "time"
    "io/ioutil"
    "encoding/json"
    "net/http"
    "net/url"
    "strings"
)

var DockerURL = flag.String("host", "unix:///var/run/docker.sock", "Docker URL")
var ShipyardURL = flag.String("url", "", "Shipyard URL")
var ShipyardKey = flag.String("key", "", "Shipyard Agent Key")
var RunInterval = flag.Int("interval", 5, "Run interval")
var Register = flag.Bool("register", false, "Register Agent with Shipyard")

type AgentData struct {
    Key     string `json:"key"`
}
type ContainerData struct {
    Container   docker.APIContainers
    Meta        *docker.Container
}

func doPostRequest(path string, body io.Reader) ([]byte, int) {
    client := &http.Client{}
    appUrl := fmt.Sprintf("%v%v", *ShipyardURL, path)
    req, _ := http.NewRequest("POST", appUrl, body)
    req.Header.Set("Authorization", fmt.Sprintf("AgentKey:%v", *ShipyardKey))
    resp, err := client.Do(req)
    if err != nil {
        fmt.Println("Error sending containers to Shipyard: ", err)
        return nil, 500
    }
    defer resp.Body.Close()
    b, _ := ioutil.ReadAll(resp.Body)
    return b, resp.StatusCode
}

func pushContainers() {
    client, err := dockerclient.NewClient(*DockerURL)
    if err != nil { panic(err) }
    opts := dockerclient.ListContainersOptions{All: true}
    containers, _ := client.ListContainers(opts)
    data := make([]ContainerData, len(containers))
    for x, c := range(containers) {
        i,_ := client.InspectContainer(c.ID)
        containerData := ContainerData{Container: c, Meta: i}
        data[x] = containerData
    }
    cnt, _ := json.Marshal(data)
    containerJSON := string(cnt)
    doPostRequest("/agent/containers/", strings.NewReader(containerJSON))
}

func pushImages() {
    client, err := dockerclient.NewClient(*DockerURL)
    if err != nil { panic(err) }
    images, _ := client.ListImages(false)
    imageData, _ := json.Marshal(images)
    imageJSON := string(imageData)
    doPostRequest("/agent/images/", strings.NewReader(imageJSON))
}

func listen(d time.Duration, f func(time.Time)) {
    for {
        f(time.Now())
        time.Sleep(d)
    }
}

// Registers with Shipyard at the specified URL
func register() {
    fmt.Println("Registering with ", *ShipyardURL)
    registerURL := fmt.Sprintf("%v/agent/register/", *ShipyardURL)
    hostname, _ := os.Hostname()
    vals := url.Values{"name": {hostname}}
    resp, _ := http.PostForm(registerURL, vals)
    defer resp.Body.Close()
    body, _ := ioutil.ReadAll(resp.Body)
    var data AgentData
    json.Unmarshal(body, &data)
    fmt.Println("Agent Key: ", data.Key)
}

func run(t time.Time) {
    go pushContainers()
    go pushImages()
}   

func main() {
    flag.Parse()
    if *ShipyardURL == "" {
        fmt.Println("Error: You must specify a Shipyard URL")
        os.Exit(1)
    }
    duration, _ := time.ParseDuration(fmt.Sprintf("%vs", *RunInterval))
    if *Register {
        register()
    } else {
        fmt.Println("Shipyard Agent", fmt.Sprintf(" (%s)", *ShipyardURL))
        listen(duration, run)
    }
}
