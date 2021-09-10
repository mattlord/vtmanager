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

	"github.com/mattlord/vtmanager/globals"
	"github.com/mattlord/vtmanager/util"
	"github.com/spf13/cobra"
)

func runCreate(cmd *cobra.Command, args []string) {
	fmt.Printf("create called with %s\n", args)
}

func runCreateCluster(cmd *cobra.Command, args []string) {
	clusterName := args[0]

	ctx := context.Background()
	cli := util.ContainerRuntimeInit(ctx, clusterName)

	if err := util.ContainerRun(ctx, cli, clusterName, globals.UNSHARDED_KS, globals.SINGLE_SHARD_RANGE, nil); err != nil {
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

	createClusterCmd.Flags().StringVarP(&globals.ClusterVersion, "version", "v", "latest", "version of vitess")
	createClusterCmd.Flags().StringVarP(&globals.MysqlVersion, "mysql-version", "m", "latest", "version of mysql")
	createClusterCmd.Flags().StringVarP(&globals.ExtraVTGateFlags, "extra-vtgate-flags", "", "", "CSV list of additional flags for vtgate")
	createClusterCmd.Flags().StringVarP(&globals.ExtraVTTabletFlags, "extra-vttablet-flags", "", "", "CSV list of additional flags for vttablet")
	createClusterCmd.Flags().StringVarP(&globals.ExtraMySQLFlags, "extra-mysqld-flags", "", "", "CSV list of additional flags for mysqld")
}
