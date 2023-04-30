package main

import (
	"context"
	"fmt"
	"io"
	"os"
    "bufio"
	"log"
    "encoding/json"
    "io/ioutil"
    "net/http"
    "path/filepath"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)


type CodeData struct {
    Code string `json:"Code"`
}

func main() {
    address := "localhost:8003"
    fs := http.FileServer(http.Dir("./"))
    http.HandleFunc("/runcode", createContainer)
    http.Handle("/", fs)
    log.Println("listen on", address)
    log.Fatal(http.ListenAndServe(address, nil))
    
}


func createContainer(w http.ResponseWriter, r *http.Request){

    var data CodeData

    // 读取请求体
    requestBody, err := ioutil.ReadAll(r.Body)
    if err != nil {
        log.Fatal(err)
    }

    // 解析 JSON 数据到结构体中
    if err := json.Unmarshal(requestBody, &data); err != nil {
        log.Fatal(err)
    }


    ex, err := os.Executable()
    if err != nil {
        panic(err)
    }

    // Get the absolute path of the executable file
    exPath := filepath.Dir(ex)

    // Get the absolute path of the project directory
    projectPath := filepath.Dir(exPath)


    // 打印结构体中的数据
    fmt.Printf("Code: %s\n", data.Code)

    os.MkdirAll(projectPath+"\\usercode",0777)

    userCodeFilepath := projectPath+"\\usercode\\main.go"
    log.Println("usercodefilepath: ",userCodeFilepath)
    f,err := os.OpenFile(userCodeFilepath,os.O_CREATE | os.O_RDWR,0644)
    if err != nil {
        log.Fatal(err)
    }
    f.WriteString(data.Code)
    f.Close()  

    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        panic(err)
    }
    defer cli.Close()

    reader, err := cli.ImagePull(ctx, "docker.io/library/golang", types.ImagePullOptions{})
    if err != nil {
        panic(err)
    }
    io.Copy(os.Stdout, reader)

    mountsSource := projectPath+"\\usercode"

    log.Println("mountsSource: ",mountsSource)

	// Mount the code directory as a volume
	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: mountsSource,
			Target: "/code",
		},
	}

    hostConfig := &container.HostConfig{
		Mounts: mounts,
        // AutoRemove: true,
	}


    config := &container.Config{
		Image:        "golang:latest",
		Cmd:          []string{"go", "run", "/code/main.go"},
		Tty:          true,
		AttachStdout: true,
	}


    resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")

    if err != nil {
        panic(err)
    }

    if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        panic(err)
    }

    statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
    select {
    case err := <-errCh:
        if err != nil {
            panic(err)
        }
    case <-statusCh:
    }

    out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true,ShowStderr: true,Follow: true})
    if err != nil {
        panic(err)
    }



    defer out.Close()

    log.Println("获取运行结果")
    // stdcopy.StdCopy(os.Stdout, os.Stderr, out)
    scanner := bufio.NewScanner(out)
    result :=""
    for scanner.Scan() {
        line := scanner.Text()
        result += line +"<br/>"
    }
  
    if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true, RemoveLinks: false}); err != nil {
        panic(err)
    }


    if _, err := fmt.Fprint(w, result); err != nil {
        panic(err)
    }
    log.Println("执行结束")

}