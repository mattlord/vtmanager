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
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/mattlord/vtmanager/util"
	"github.com/spf13/cobra"
)

func runCreate(cmd *cobra.Command, args []string) {
	fmt.Printf("create called with %s\n", args)
}

func runCreateCluster(cmd *cobra.Command, args []string) {
	clusterName = args[0]
	etcdAddr := fmt.Sprintf("http://etcd-%s:%d", clusterName, ETCD_PORT)
	networkName := fmt.Sprintf("net-%s", clusterName)
	topoGobalServerAddrFlag := fmt.Sprintf("-topo_global_server_address=%s", etcdAddr)
	vtctldServer := fmt.Sprintf("vtctld-%s:%d", clusterName, VT_GRPC_PORT)
	vtctldServerFlag := fmt.Sprintf("-server=%s", vtctldServer)
	ctx := context.Background()

	cli := initContainerContext(ctx)

	for _, baseVitessContainer := range vitessContainers {
		baseVitessContainer.netconfig.EndpointsConfig["NetworkID"] = &network.EndpointSettings{
			NetworkID: networkName,
		}

		for n := 1; n <= baseVitessContainer.count; n++ {
			// let's make a copy as we will add some dynamic flags
			vitessContainer := *baseVitessContainer
			nameAppendix := clusterName
			if baseVitessContainer.count > 1 {
				nameAppendix += fmt.Sprintf("-%d", n)
			}
			vitessContainer.config.Hostname = fmt.Sprintf("%s-%s", vitessContainer.name, nameAppendix)
			vitessContainer.hostconfig.RestartPolicy = container.RestartPolicy{"on-failure", 100}

			if strings.HasPrefix(vitessContainer.name, "vt") {
				addlVitessFlags := []string{
					topoGobalServerAddrFlag,
					fmt.Sprintf("-port=%d", VT_WEB_PORT),
					fmt.Sprintf("-grpc_port=%d", VT_GRPC_PORT),
				}
				vitessContainer.config.Cmd = append(vitessContainer.config.Cmd, addlVitessFlags...)

				if vitessContainer.name == "vttablet" {
					addlTabletFlags := []string{
						fmt.Sprintf("-vtctld_addr=http://%s/", vtctldServer),
						fmt.Sprintf("-db_host=mysqld-%s", nameAppendix),
						fmt.Sprintf("%s-%d", baseTabletAliasFlag, n),
						fmt.Sprintf("-tablet_hostname=%s", vitessContainer.config.Hostname),
					}
					vitessContainer.config.Cmd = append(vitessContainer.config.Cmd, addlTabletFlags...)
				}
			}

			resp, err := cli.ContainerCreate(ctx, &vitessContainer.config, &vitessContainer.hostconfig,
				&vitessContainer.netconfig, nil, vitessContainer.config.Hostname)
			if err != nil {
				fmt.Printf("Error when creating container %s: %s\n", vitessContainer.config.Hostname, err)
			}

			if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
				fmt.Printf("Error when starting container %s: %s\n", vitessContainer.config.Hostname, err)
			}

			execRetries := 20
			var execExitCode int

			// mysqld takes a while to initialize and restart, and it needs to be running before we can
			// start the vttablet...
			if baseVitessContainer.name == "mysqld" && resp.ID != "" {
				spinr := spinner.New(spinner.CharSets[SPANNER_CHARSET], 100*time.Millisecond)
				spinr.FinalMSG = "done!"

				fmt.Printf("Waiting for mysqld on %s to initialize and be ready to serve queries\n", vitessContainer.config.Hostname)
				spinr.Start()
				time.Sleep(60 * time.Second) // we know it typically takes 60+ seconds
				for retryCount := 0; retryCount < execRetries; retryCount++ {
					if execExitCode = util.ContainerExec(ctx, cli, resp.ID, []string{"mysql", "-u", "root", "-BNe", "'select 1'"}); execExitCode == 0 {
						break
					}
					time.Sleep(10 * time.Second)
				}
				spinr.Stop()
				fmt.Println("")

				if execExitCode != 0 {
					panic(fmt.Sprintf("mysqld did not become ready to serve queries after %d retries, giving up!\n", execRetries))
				}
			}

			// We need to create the cell
			if baseVitessContainer.name == "vtctld" && resp.ID != "" {
				for {
					if execExitCode = util.ContainerExec(ctx, cli, resp.ID, []string{"/vt/bin/vtctlclient", vtctldServerFlag, "AddCellInfo", "-root=/vitess/local", fmt.Sprintf("-server_address=%s", etcdAddr), "local"}); execExitCode == 0 {
						break
					}
					time.Sleep(1 * time.Second)
				}

				if execExitCode != 0 {
					panic(fmt.Sprintf("Cell %s could not be created after %d retries, giving up!\n", VITESS_CELL, execRetries))
				}
			}

			// Let's init the shard primary
			if baseVitessContainer.name == "vttablet" && resp.ID != "" {
				keyspaceShard := fmt.Sprintf("%s/%s", UNSHARDED_KS, SINGLE_SHARD_RANGE)
				tabletAlias := fmt.Sprintf("%s-%d", VITESS_CELL, n)
				tabletStatusURL := fmt.Sprintf("http://vttablet-%s:%d/debug/status", nameAppendix, VT_WEB_PORT)

				// Let's first ensure that the tablet is fully functional, then make it the primary
				for retryCount := 0; retryCount < execRetries; retryCount++ {
					if util.ContainerExec(ctx, cli, resp.ID, []string{"curl", "-I", tabletStatusURL}) == 0 &&
						util.ContainerExec(ctx, cli, resp.ID, []string{"/vt/bin/vtctlclient", vtctldServerFlag, "InitShardMaster", "-force", keyspaceShard, tabletAlias}) == 0 {
						execExitCode = 0
						break
					}
					time.Sleep(1 * time.Second)
				}

				if execExitCode != 0 {
					panic(fmt.Sprintf("Could not initialize %s as shard primary for %s after %d retries, giving up!\n", tabletAlias, keyspaceShard, execRetries))
				}
			}
		}
	}
}

func initContainerContext(ctx context.Context) *client.Client {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	fmt.Println("Pulling up to date container images")
	spinr := spinner.New(spinner.CharSets[SPANNER_CHARSET], 100*time.Millisecond)
	spinr.Start()
	spinr.FinalMSG = "done!"

	for _, baseVitessContainer := range vitessContainers {
		if baseVitessContainer.name == "mysqld" && mysqlVersion != "latest" {
			baseVitessContainer.config.Image += ":" + mysqlVersion
		}

		if strings.HasPrefix(baseVitessContainer.name, "vt") && clusterVersion != "latest" {
			baseVitessContainer.config.Image += ":v" + clusterVersion
		}

		_, err := cli.ImagePull(ctx, baseVitessContainer.config.Image, types.ImagePullOptions{})

		if err != nil {
			fmt.Printf("Container image %s not found\n", baseVitessContainer.config.Image)
			os.Exit(1)
		}
		//io.Copy(os.Stdout, reader)
	}

	spinr.Stop()
	fmt.Println("")

	networkName := fmt.Sprintf("net-%s", clusterName)
	if _, err := cli.NetworkCreate(ctx, networkName, types.NetworkCreate{CheckDuplicate: true}); err != nil {
		errStr := fmt.Sprintf("%s", err)
		if !strings.HasSuffix(errStr, "already exists") {
			fmt.Printf("Error when creating network %s: %s\n", networkName, err)
			os.Exit(1)
		}
	}

	return cli
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
	createClusterCmd.Flags().StringVarP(&mysqlVersion, "mysql-version", "m", "latest", "version of mysql")
}
