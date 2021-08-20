/*
Copyright © 2021 Matt Lord <mattalord@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/spf13/cobra"
)

var (
	clusterVersion string
)

const (
	VITESS_LITE_CONTAINER_IMAGE = "docker.io/vitess/lite"
	ETCD_CONTAINER_IMAGE        = "docker.io/bitnami/etcd:3.5.0"
)

func runCreate(cmd *cobra.Command, args []string) {
	fmt.Printf("create called with %s\n", args)
}

func runCreateCluster(cmd *cobra.Command, args []string) {
	fmt.Printf("create cluster called with %s\n", args)

	clusterName := args[0]
	containerImage := VITESS_LITE_CONTAINER_IMAGE

	if clusterVersion != "latest" {
		containerImage += ":v" + clusterVersion
	}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	for _, image := range []string{ETCD_CONTAINER_IMAGE, containerImage} {
		reader, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
		if err != nil {
			fmt.Printf("ERROR: container image %s not found\n", image)
			os.Exit(1)
		}
		io.Copy(os.Stdout, reader)
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: containerImage,
		Cmd:   []string{"/vt/bin/vtgate", "-version", "2>/dev/null"},
		Tty:   false,
	}, nil, nil, nil, fmt.Sprintf("cluster_%s", clusterName))
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

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		panic(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	if err := cli.ContainerStop(ctx, resp.ID, nil); err != nil {
		panic(err)
	}

	if err := cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{}); err != nil {
		panic(err)
	}
}

func runCreateKeyspace(cmd *cobra.Command, args []string) {
	fmt.Printf("create keyspace called with %s\n", args)
}

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Vitess objects",
	Long:  ``,
	Run:   runCreate,
}

var createClusterCmd = &cobra.Command{
	Use:   "cluster [name]",
	Short: "Create Vitess cluster",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	Run:   runCreateCluster,
}

var createKeyspaceCmd = &cobra.Command{
	Use:   "keyspace",
	Short: "Create Vitess keyspace",
	Long:  ``,
	Run:   runCreateKeyspace,
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.AddCommand(createClusterCmd)
	createCmd.AddCommand(createKeyspaceCmd)

	createClusterCmd.Flags().StringVarP(&clusterVersion, "version", "v", "latest", "version of vitess")
}
