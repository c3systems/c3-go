package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/docker/go-connections/tlsconfig"
)

// Client ...
type Client struct {
	client *client.Client
}

// NewClient ...
func NewClient() *Client {
	return newEnvClient()
}

// newClient ...
func newClient() *Client {
	httpclient := &http.Client{}

	if dockerCertPath := os.Getenv("DOCKER_CERT_PATH"); dockerCertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv("DOCKER_TLS_VERIFY") == "",
		}
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			log.Fatal(err)
		}

		httpclient.Transport = &http.Transport{
			TLSClientConfig: tlsc,
		}
	}

	host := os.Getenv("DOCKER_HOST")
	version := os.Getenv("DOCKER_VERSION")

	if host == "" {
		log.Fatal("DOCKER_HOST is required")
	}

	if version == "" {
		version = dockerVersionFromCLI()
		if version == "" {
			log.Fatal("DOCKER_VERSION is required")
		}
	}

	cl, err := client.NewClient(host, version, httpclient, nil)
	if err != nil {
		log.Fatal(err)
	}

	return &Client{
		client: cl,
	}
}

// newEnvClient ...
func newEnvClient() *Client {
	cl, err := client.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}

	return &Client{
		client: cl,
	}
}

// ImageSummary ....
type ImageSummary struct {
	ID   string
	Size int64
}

// ListImages ...
func (s *Client) ListImages() ([]*ImageSummary, error) {
	images, err := s.client.ImageList(context.Background(), types.ImageListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	var summaries []*ImageSummary
	for _, img := range images {
		summaries = append(summaries, &ImageSummary{
			ID:   img.ID,
			Size: img.Size,
		})
	}

	return summaries, nil
}

// PullImage ...
func (s *Client) PullImage(imageID string) error {
	reader, err := s.client.ImagePull(context.Background(), imageID, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, reader)
	return nil
}

// PushImage ...
func (s *Client) PushImage(imageID string) error {
	reader, err := s.client.ImagePush(context.Background(), imageID, types.ImagePushOptions{
		RegistryAuth: "123", // if no auth, then any value is required
	})
	if err != nil {
		return err
	}
	io.Copy(os.Stdout, reader)
	return nil
}

// RunContainerConfig ...
type RunContainerConfig struct {
	// container:host
	Volumes map[string]string
	Ports   map[string]string
}

// RunContainer ...
func (s *Client) RunContainer(imageID string, cmd []string, config *RunContainerConfig) (string, error) {
	if config == nil {
		config = &RunContainerConfig{}
	}

	dockerConfig := &container.Config{
		Image:        imageID,
		Cmd:          cmd,
		Tty:          false,
		Volumes:      map[string]struct{}{},
		ExposedPorts: map[nat.Port]struct{}{},
	}

	hostConfig := &container.HostConfig{
		Binds:        nil,
		PortBindings: map[nat.Port][]nat.PortBinding{},
		AutoRemove:   true,
		IpcMode:      "",
		Privileged:   false,
		Mounts:       []mount.Mount{},
	}

	if len(config.Volumes) > 0 {
		for k, v := range config.Volumes {
			dockerConfig.Volumes[k] = struct{}{}

			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Type:     "bind",
				Source:   v,
				Target:   k,
				ReadOnly: false,
			})
		}
	}

	if len(config.Ports) > 0 {
		for k, v := range config.Ports {
			t, err := nat.NewPort("tcp", k)
			if err != nil {
				return "", err
			}
			dockerConfig.ExposedPorts[t] = struct{}{}
			hostConfig.PortBindings[t] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: v,
				},
			}
		}
	}

	resp, err := s.client.ContainerCreate(context.Background(), dockerConfig, hostConfig, nil, "")

	/*
			container.Config{
				Hostname:     "",
				Domainname:   "",
				User:         "",
				AttachStdin:  false,
				AttachStdout: false,
				AttachStderr: false,
				ExposedPorts: map[nat.Port]struct{}{
					"": {},
				},
				Tty:       false,
				OpenStdin: false,
				StdinOnce: false,
				Env:       nil,
				Cmd:       nil,
				Healthcheck: &container.HealthConfig{
					Test:     nil,
					Interval: 0,
					Timeout:  0,
					Retries:  0,
				},
				ArgsEscaped: false,
				Image:       "",
				Volumes: map[string]struct{}{
					"": {},
				},
				WorkingDir:      "",
				Entrypoint:      nil,
				NetworkDisabled: false,
				MacAddress:      "",
				OnBuild:         nil,
				Labels: map[string]string{
					"": "",
				},
				StopSignal:  "",
				StopTimeout: nil,
				Shell:       nil,
			}

		&container.HostConfig{
			Binds:           nil,
			ContainerIDFile: "",
			LogConfig: container.LogConfig{
				Type: "",
				Config: map[string]string{
					"": "",
				},
			},
			NetworkMode: "",
			PortBindings: map[nat.Port][]nat.PortBinding{
				"": nil,
			},
			RestartPolicy: container.RestartPolicy{
				Name:              "",
				MaximumRetryCount: 0,
			},
			AutoRemove:      false,
			VolumeDriver:    "",
			VolumesFrom:     nil,
			CapAdd:          nil,
			CapDrop:         nil,
			DNS:             nil,
			DNSOptions:      nil,
			DNSSearch:       nil,
			ExtraHosts:      nil,
			GroupAdd:        nil,
			IpcMode:         "",
			Cgroup:          "",
			Links:           nil,
			OomScoreAdj:     0,
			PidMode:         "",
			Privileged:      false,
			PublishAllPorts: false,
			ReadonlyRootfs:  false,
			SecurityOpt:     nil,
			StorageOpt: map[string]string{
				"": "",
			},
			Tmpfs: map[string]string{
				"": "",
			},
			UTSMode:    "",
			UsernsMode: "",
			ShmSize:    0,
			Sysctls: map[string]string{
				"": "",
			},
			Runtime: "",
			ConsoleSize: [2]uint{
				0,
				0,
			},
			Isolation: "",
			Resources: container.Resources{
				CPUShares:            0,
				Memory:               0,
				NanoCPUs:             0,
				CgroupParent:         "",
				BlkioWeight:          0,
				BlkioWeightDevice:    nil,
				BlkioDeviceReadBps:   nil,
				BlkioDeviceWriteBps:  nil,
				BlkioDeviceReadIOps:  nil,
				BlkioDeviceWriteIOps: nil,
				CPUPeriod:            0,
				CPUQuota:             0,
				CPURealtimePeriod:    0,
				CPURealtimeRuntime:   0,
				CpusetCpus:           "",
				CpusetMems:           "",
				Devices:              nil,
				DiskQuota:            0,
				KernelMemory:         0,
				MemoryReservation:    0,
				MemorySwap:           0,
				MemorySwappiness:     nil,
				OomKillDisable:       nil,
				PidsLimit:            0,
				Ulimits:              nil,
				CPUCount:             0,
				CPUPercent:           0,
				IOMaximumIOps:        0,
				IOMaximumBandwidth:   0,
			},
			Mounts:   nil,
			Init:     nil,
			InitPath: "",
		}
	*/
	if err != nil {
		return "", err
	}

	err = s.client.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{})

	if err != nil {
		return "", err
	}

	log.Printf("running container %s", resp.ID)

	return resp.ID, nil
}

// StopContainer ...
func (s *Client) StopContainer(containerID string) error {
	log.Printf("stopping container %s", containerID)
	err := s.client.ContainerStop(context.Background(), containerID, nil)
	if err != nil {
		return err
	}

	log.Println("container stopped")
	return nil
}

// InspectContainer ...
func (s *Client) InspectContainer(containerID string) (types.ContainerJSON, error) {
	info, err := s.client.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return types.ContainerJSON{}, err
	}

	return info, nil
}

// ContainerExec ...
func (s *Client) ContainerExec(containerID string, cmd []string) (io.Reader, error) {
	id, err := s.client.ContainerExecCreate(context.Background(), containerID, types.ExecConfig{
		AttachStdout: true,
		Cmd:          cmd,
	})

	log.Println("exec ID", id.ID)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.ContainerExecAttach(context.Background(), id.ID, types.ExecConfig{})
	if err != nil {
		return nil, err
	}

	return resp.Reader, nil
}

// ReadImage ...
func (s *Client) ReadImage(imageID string) (io.Reader, error) {
	return s.client.ImageSave(context.Background(), []string{imageID})
}

// LoadImage ...
func (s *Client) LoadImage(input io.Reader) error {
	output, err := s.client.ImageLoad(context.Background(), input, false)
	if err != nil {
		return err
	}

	//io.Copy(os.Stdout, output)
	fmt.Println(output)
	body, err := ioutil.ReadAll(output.Body)
	fmt.Println(string(body))

	return err
}

// LoadImageByFilepath ...
func (s *Client) LoadImageByFilepath(filepath string) error {
	input, err := os.Open(filepath)
	if err != nil {
		return err
	}
	return s.LoadImage(input)
}

func dockerVersionFromCLI() string {
	cmd := `docker version --format="{{.Client.APIVersion}}"`
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}
