package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/djherbis/buffer"
)

func CreateBackup(mb int) (io.Reader, error) {
	args := []string{
		"if=/dev/zero",
		"bs=1m",
		fmt.Sprintf("count=%d", mb),
	}
	cmd := exec.Command("dd", args...)

	// buffer 32MB to memory, after that buffer to 64MB chunked files
	backup := buffer.NewUnboundedBuffer(32*1024*1024, 64*1024*1024)
	cmd.Stdout = backup

	var capture bytes.Buffer
	cmd.Stderr = &capture

	err := cmd.Run()
	if err != nil {
		return nil, errors.New(capture.String())
	}

	return backup, nil
}

func WatchDir(path string) {
	c := time.Tick(100 * time.Millisecond)
	for _ = range c {
		fmt.Println("--- files ---")
		files, _ := os.ReadDir(path)
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "buffer") {
				continue
			}
			info, _ := file.Info()
			fmt.Printf("%s %d\n", file.Name(), info.Size())
		}
	}
}

func main() {
	backup, err := CreateBackup(1024)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("foo.iso")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	go WatchDir(os.TempDir())

	n, err := io.Copy(f, backup)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(n)
	time.Sleep(1 * time.Second)
}
