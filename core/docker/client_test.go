package docker

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var (
	testImage    = "hello-world"
	testImageTar = "./test_data/hello-world.tar"
)

func TestNew(t *testing.T) {
	t.Parallel()
	client := NewClient()
	if client == nil {
		t.Error("expected instance")
	}
}

func TestListImages(t *testing.T) {
	t.Parallel()
	client := NewClient()
	images, err := client.ListImages()
	if err != nil {
		t.Error(err)
	}

	for _, image := range images {
		if len(image.ID) == 0 {
			t.Error("expected image ID")
		}
		if image.Size <= 0 {
			t.Error("expected image size")
		}
	}
}

func TestHasImage(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}
	hasImage, err := client.HasImage(testImage)
	if err != nil {
		t.Error(err)
	}
	if !hasImage {
		t.Error("expected to have image")
	}
}

func TestPullImage(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}
}

func TestReadImage(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}
	reader, err := client.ReadImage(testImage)
	if err != nil {
		t.Error(err)
	}

	io.Copy(os.Stdout, reader)
}

func TestLoadImage(t *testing.T) {
	t.Parallel()
	client := NewClient()
	input, err := os.Open(testImageTar)
	if err != nil {
		t.Error(err)
	}
	err = client.LoadImage(input)
	if err != nil {
		t.Error(err)
	}
}

func TestLoadImageByFilepath(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.LoadImageByFilepath(testImageTar)
	if err != nil {
		t.Error(err)
	}
}

func TestTagImage(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}
	newTag := "my-image:mytag"
	err = client.TagImage(testImage, newTag)
	if err != nil {
		t.Error(err)
	}

	images, err := client.ListImages()
	if err != nil {
		t.Error(err)
	}

	var hasImage bool
	for _, image := range images {
		for _, tag := range image.Tags {
			if tag == newTag {
				hasImage = true
				break
			}
		}
	}

	if !hasImage {
		t.Error("expected image tag")
	}
}

func TestRemoveImage(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}

	err = client.RemoveImage(testImage)
	if err != nil {
		t.Error(err)
	}
}

func TestRemoveAllImages(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.RemoveAllImages()
	if err != nil {
		t.Error(err)
	}

	images, err := client.ListImages()
	if err != nil {
		t.Error(err)
	}

	if len(images) != 0 {
		t.Error("expected number of images to be 0")
	}
}

func TestRunContainer(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}
	containerID, err := client.RunContainer(testImage, []string{}, nil)
	if err != nil {
		t.Error(err)
	}

	if containerID == "" {
		t.Error("expected container ID")
	}
}

func TestStopContainer(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}
	containerID, err := client.RunContainer(testImage, []string{}, nil)
	if err != nil {
		t.Error(err)
	}

	err = client.StopContainer(containerID)
	if err != nil {
		t.Error(err)
	}
}

func TestInspectContainer(t *testing.T) {
	t.Parallel()
	client := NewClient()
	err := client.PullImage(testImage)
	if err != nil {
		t.Error(err)
	}
	containerID, err := client.RunContainer(testImage, []string{}, nil)
	if err != nil {
		t.Error(err)
	}
	info, err := client.InspectContainer(containerID)
	if err != nil {
		t.Error(err)
	}

	if info.ID != containerID {
		t.Error("expected id to match")
	}

	err = client.StopContainer(containerID)
	if err != nil {
		t.Error(err)
	}
}

func TestCopyToContainerAndCopyFromContainer(t *testing.T) {
	t.Parallel()
	client := NewClient()
	imageName := "alpine:latest"
	err := client.PullImage(imageName)
	if err != nil {
		t.Error(err)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var files = []struct {
		Name, Body string
	}{
		{"state.txt", "hello world"},
	}
	for _, file := range files {
		hdr := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Error(err)
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			t.Error(err)
		}
	}
	defer tw.Close()

	r := bytes.NewReader(buf.Bytes())
	containerID, err := client.RunContainer(imageName, []string{"tail", "-f", "/dev/null"}, nil)
	if err != nil {
		t.Error(err)
	}
	err = client.CopyToContainer(containerID, "/tmp", r)
	if err != nil {
		t.Error(err)
	}

	out, err := client.CopyFromContainer(containerID, "/tmp/state.txt")
	if err != nil {
		t.Error(err)
	}

	tr := tar.NewReader(out)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			t.Error(err)
		}
		if hdr.Name != "state.txt" {
			t.Error(err)
		}
		b, err := ioutil.ReadAll(tr)
		if err != nil {
			t.Error(err)
		}

		if string(b) != "hello world" {
			t.Error("expected match")
		}
	}

	err = client.StopContainer(containerID)
	if err != nil {
		t.Error(err)
	}
}

func TestDockerVersionFromCLI(t *testing.T) {
	t.Parallel()
	version := dockerVersionFromCLI()
	if version == "" {
		t.Error("expected version to not be empty")
	}
}

func untar(tr *tar.Reader, dst string) error {
	//tr := tar.NewReader(reader)

	for {
		header, err := tr.Next()

		fmt.Printf("Contents of %v:\n", header)
		switch {
		// no more files
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		target := filepath.Join(dst, header.Name)

		switch header.Typeflag {
		// create directory if doesn't exit
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}
		// create file
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy contents to file
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		}
	}
}
