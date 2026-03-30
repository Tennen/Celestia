package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/chentianyu/celestia/internal/coreapi"
)

const defaultImage = "celestia-hikvision-plugin:latest"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	pluginPort := strings.TrimSpace(os.Getenv("CELESTIA_PLUGIN_PORT"))
	if pluginPort == "" {
		return errors.New("CELESTIA_PLUGIN_PORT is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	containerName := strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_CONTAINER_NAME"))
	if containerName == "" {
		containerName = "celestia-hikvision-plugin-" + strings.ReplaceAll(pluginPort, ":", "-")
	}
	image := strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_DOCKER_IMAGE"))
	if image == "" {
		image = defaultImage
	}

	dockerArgs := buildDockerArgs(pluginPort, containerName, image)
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if ctx.Err() != nil {
		cleanupContainer(containerName)
		return nil
	}
	if err != nil {
		cleanupContainer(containerName)
		return err
	}
	return nil
}

func buildDockerArgs(pluginPort, containerName, image string) []string {
	args := []string{"run", "--rm", "--name", containerName, "--init"}
	if addHostEnabled() {
		args = append(args, "--add-host", "host.docker.internal:host-gateway")
	}
	if platform := strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_DOCKER_PLATFORM")); platform != "" {
		args = append(args, "--platform", platform)
	}
	networkMode := strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_DOCKER_NETWORK"))
	if networkMode != "" {
		args = append(args, "--network", networkMode)
	}
	if networkMode != "host" {
		args = append(args, "-p", fmt.Sprintf("127.0.0.1:%s:%s", pluginPort, pluginPort))
	}

	args = append(args,
		"-e", "CELESTIA_PLUGIN_PORT="+pluginPort,
		"-e", "CELESTIA_PLUGIN_BIND_HOST=0.0.0.0",
	)
	if coreAddr := strings.TrimSpace(os.Getenv(coreapi.EnvCoreAddr)); coreAddr != "" {
		args = append(args, "-e", coreapi.EnvCoreAddr+"="+remapCoreAddr(coreAddr))
	}
	if sdkLibDir := strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_SDK_LIB_DIR")); sdkLibDir != "" {
		args = append(args, "-e", "CELESTIA_HIKVISION_SDK_LIB_DIR="+sdkLibDir)
	}

	args = append(args, image)
	return args
}

func remapCoreAddr(addr string) string {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil || port == "" {
		return addr
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	switch host {
	case "", "127.0.0.1", "localhost", "::1":
		return net.JoinHostPort("host.docker.internal", port)
	default:
		return addr
	}
}

func addHostEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_DOCKER_ADD_HOST_GATEWAY")))
	switch value {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func cleanupContainer(containerName string) {
	if containerName == "" {
		return
	}
	cmd := exec.Command("docker", "rm", "-f", containerName)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
