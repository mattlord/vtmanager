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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

func runDeleteCluster(cmd *cobra.Command, args []string) {
	fmt.Printf("delete cluster called with %s\n", args)

	clusterName := args[0]

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	for _, baseName := range []string{"vtgate", "vtctld", "vttablet", "mysqld", "etcd"} {
		containerName := fmt.Sprintf("%s-%s", baseName, clusterName)
		if err := cli.ContainerStop(ctx, containerName, nil); err != nil {
			fmt.Println(err)
		}

		if err := cli.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{}); err != nil {
			fmt.Println(err)
		}
	}

	if err := cli.NetworkRemove(ctx, "net-"+clusterName); err != nil {
		fmt.Println(err)
	}
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete Vitess objects",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("delete called")
	},
}

var deleteClusterCmd = &cobra.Command{
	Use:   "cluster [name]",
	Short: "Delete a Vitess cluster",
	Long:  ``,
	Args:  cobra.ExactArgs(1),
	Run:   runDeleteCluster,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.AddCommand(deleteClusterCmd)
}
