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

package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/containerd/containerd/pkg/cri/util"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/mattlord/vtmanager/globals"
)

const (
	EXEC_CREATE_FAILED  = 100
	EXEC_ATTACH_FAILED  = 101
	EXEC_START_FAILED   = 102
	EXEC_INSPECT_FAILED = 103
)

func ContainerRuntimeInit(ctx context.Context, clusterName string) *client.Client {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	fmt.Println("Pulling up to date container images")
	spinr := spinner.New(spinner.CharSets[globals.SPINNER_CHARSET], 100*time.Millisecond)
	spinr.Start()
	spinr.FinalMSG = "done!"

	for _, baseVitessContainer := range globals.VitessContainers {
		if baseVitessContainer.Name == "mysqld" && globals.MysqlVersion != "latest" {
			baseVitessContainer.Config.Image += ":" + globals.MysqlVersion
		}

		if strings.HasPrefix(baseVitessContainer.Name, "vt") && globals.ClusterVersion != "latest" {
			baseVitessContainer.Config.Image += ":v" + globals.ClusterVersion
		}

		reader, err := cli.ImagePull(ctx, baseVitessContainer.Config.Image, types.ImagePullOptions{})

		if err != nil {
			fmt.Printf("Container image %s not found\n", baseVitessContainer.Config.Image)
			os.Exit(1)
		}

		defer reader.Close()
		//io.Copy(os.Stdout, reader)
		io.ReadAll(reader)
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

/*
  If you don't want to run all vitess container types, then pass a slice of container names
*/
func ContainerRun(ctx context.Context, cli *client.Client, clusterName string, keyspace string, shard string, containers []string) error {
	etcdAddr := fmt.Sprintf("http://etcd-%s:%d", clusterName, globals.ETCD_PORT)
	networkName := fmt.Sprintf("net-%s", clusterName)
	topoGobalServerAddrFlag := fmt.Sprintf("-topo_global_server_address=%s", etcdAddr)
	vtctldServer := fmt.Sprintf("vtctld-%s:%d", clusterName, globals.VT_GRPC_PORT)
	vtctldServerFlag := fmt.Sprintf("-server=%s", vtctldServer)

	for _, baseVitessContainer := range globals.VitessContainers {
		if containers != nil && !util.InStringSlice(containers, baseVitessContainer.Name) {
			continue
		}

		baseVitessContainer.Netconfig.EndpointsConfig["NetworkID"] = &network.EndpointSettings{
			NetworkID: networkName,
		}

		for n := 1; n <= baseVitessContainer.Count; n++ {
			// let's make a copy as we will add some dynamic flags
			vitessContainer := *baseVitessContainer
			nameAppendix := clusterName
			if baseVitessContainer.Count > 1 {
				nameAppendix += fmt.Sprintf("-%d", n)
			}
			vitessContainer.Config.Hostname = fmt.Sprintf("%s-%s", vitessContainer.Name, nameAppendix)
			vitessContainer.Hostconfig.RestartPolicy = container.RestartPolicy{"on-failure", 100}

			if strings.HasPrefix(vitessContainer.Name, "vt") {
				addlVitessFlags := []string{
					topoGobalServerAddrFlag,
					fmt.Sprintf("-port=%d", globals.VT_WEB_PORT),
					fmt.Sprintf("-grpc_port=%d", globals.VT_GRPC_PORT),
				}
				vitessContainer.Config.Cmd = append(vitessContainer.Config.Cmd, addlVitessFlags...)

				if vitessContainer.Name == "vttablet" {
					addlTabletFlags := []string{
						fmt.Sprintf("-init_keyspace=%s", keyspace),
						fmt.Sprintf("-init_shard=%s", shard),
						fmt.Sprintf("-vtctld_addr=http://%s/", vtctldServer),
						fmt.Sprintf("-db_host=mysqld-%s", nameAppendix),
						fmt.Sprintf("-init_db_name_override=%s", keyspace),
						fmt.Sprintf("%s-%d", globals.BaseTabletAliasFlag, n),
						fmt.Sprintf("-tablet_hostname=%s", vitessContainer.Config.Hostname),
					}
					if globals.ExtraVTTabletFlags != "" {
						addlTabletFlags = append(addlTabletFlags, strings.Split(globals.ExtraVTTabletFlags, ",")...)
					}
					vitessContainer.Config.Cmd = append(vitessContainer.Config.Cmd, addlTabletFlags...)
				}

				if vitessContainer.Name == "vtgate" {
					addlGatewayFlags := []string{
						fmt.Sprintf("-mysql_server_version=%s", globals.MysqlVersion),
					}
					if globals.ExtraVTGateFlags != "" {
						addlGatewayFlags = append(addlGatewayFlags, strings.Split(globals.ExtraVTGateFlags, ",")...)
					}
					vitessContainer.Config.Cmd = append(vitessContainer.Config.Cmd, addlGatewayFlags...)
				}
			}

			if vitessContainer.Name == "mysqld" {
				addlMysqldFlags := []string{
					fmt.Sprintf("--server-id=%d", n),
				}
				if globals.ExtraMySQLFlags != "" {
					addlMysqldFlags = append(addlMysqldFlags, strings.Split(globals.ExtraMySQLFlags, ",")...)
				}
				vitessContainer.Config.Cmd = append(vitessContainer.Config.Cmd, addlMysqldFlags...)
				addlMysqldEnvs := []string{
					fmt.Sprintf("MYSQL_DATABASE=%s", keyspace),
				}
				vitessContainer.Config.Env = append(vitessContainer.Config.Env, addlMysqldEnvs...)
			}

			resp, err := cli.ContainerCreate(ctx, &vitessContainer.Config, &vitessContainer.Hostconfig,
				&vitessContainer.Netconfig, nil, vitessContainer.Config.Hostname)
			if err != nil {
				fmt.Printf("Error when creating container %s: %s\n", vitessContainer.Config.Hostname, err)
			}

			if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
				fmt.Printf("Error when starting container %s: %s\n", vitessContainer.Config.Hostname, err)
			}

			execRetries := 20
			var execExitCode int

			// mysqld takes a while to initialize and restart, and it needs to be running before we can
			// start the vttablet...
			if baseVitessContainer.Name == "mysqld" && resp.ID != "" {
				spinr := spinner.New(spinner.CharSets[globals.SPINNER_CHARSET], 100*time.Millisecond)

				spinr.FinalMSG = "done!"

				fmt.Printf("Waiting for mysqld on %s to initialize and be ready to serve queries\n", vitessContainer.Config.Hostname)
				spinr.Start()
				time.Sleep(60 * time.Second) // we know it typically takes 60+ seconds
				for retryCount := 0; retryCount < execRetries; retryCount++ {
					if execExitCode = ContainerExec(ctx, cli, resp.ID, []string{"mysql", "-u", "root", "-BNe", "'select 1'"}); execExitCode == 0 {
						break
					}
					time.Sleep(10 * time.Second)
				}
				spinr.Stop()
				fmt.Println()

				if execExitCode != 0 {
					panic(fmt.Sprintf("mysqld did not become ready to serve queries after %d retries, giving up!\n", execRetries))
				}
			}

			// We need to create the cell
			if baseVitessContainer.Name == "vtctld" && resp.ID != "" {
				for {
					if execExitCode = ContainerExec(ctx, cli, resp.ID, []string{"/vt/bin/vtctlclient", vtctldServerFlag, "AddCellInfo", "-root=/vitess/local", fmt.Sprintf("-server_address=%s", etcdAddr), "local"}); execExitCode == 0 {
						break
					}
					time.Sleep(1 * time.Second)
				}

				if execExitCode != 0 {
					panic(fmt.Sprintf("Cell %s could not be created after %d retries, giving up!\n", globals.VITESS_CELL, execRetries))
				}
			}

			// Let's init the shard primary
			if baseVitessContainer.Name == "vttablet" && resp.ID != "" {
				keyspaceShard := fmt.Sprintf("%s/%s", globals.UNSHARDED_KS, globals.SINGLE_SHARD_RANGE)
				tabletAlias := fmt.Sprintf("%s-1", globals.VITESS_CELL)
				tabletStatusURL := fmt.Sprintf("http://vttablet-%s-1:%d/debug/status", clusterName, globals.VT_WEB_PORT)

				// Let's first ensure that the tablet is fully functional, then make it the primary
				for retryCount := 0; retryCount < execRetries; retryCount++ {
					if ContainerExec(ctx, cli, resp.ID, []string{"curl", "-s", "-f", "-I", tabletStatusURL}) == 0 &&
						ContainerExec(ctx, cli, resp.ID, []string{"/vt/bin/vtctlclient", vtctldServerFlag, "InitShardMaster", "-force", keyspaceShard, tabletAlias}) == 0 {
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

	return nil
}

func ContainerExec(ctx context.Context, cli client.APIClient, id string, cmd []string) int {
	execConfig := types.ExecConfig{
		AttachStdout: false,
		AttachStderr: false,
		Cmd:          cmd,
	}

	//fmt.Printf("Running %s in the %s container...\n", cmd, id)
	cresp, err := cli.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return EXEC_CREATE_FAILED
	}
	execID := cresp.ID

	aresp, err := cli.ContainerExecAttach(ctx, execID, types.ExecStartCheck{})
	if err != nil {
		return EXEC_ATTACH_FAILED
	}
	defer aresp.Close()

	err = cli.ContainerExecStart(ctx, execID, types.ExecStartCheck{})
	if err != nil {
		return EXEC_START_FAILED
	}

	iresp, err := cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		return EXEC_INSPECT_FAILED
	}

	return iresp.ExitCode
}

/*
  If you don't want to run all vitess container types, then pass a slice of container names
*/
func ContainerDelete(ctx context.Context, cli *client.Client, clusterName string, containers []string) error {
	fmt.Printf("Stopping and deleting containers for the %s cluster\n", clusterName)
	spinr := spinner.New(spinner.CharSets[globals.SPINNER_CHARSET], 100*time.Millisecond)
	spinr.Start()
	spinr.FinalMSG = "done!"

	for _, baseVitessContainer := range globals.VitessContainers {
		if containers != nil && !util.InStringSlice(containers, baseVitessContainer.Name) {
			continue
		}

		for n := 1; n <= baseVitessContainer.Count; n++ {
			containerName := fmt.Sprintf("%s-%s", baseVitessContainer.Name, clusterName)
			if baseVitessContainer.Count > 1 {
				containerName += fmt.Sprintf("-%d", n)
			}

			if err := cli.ContainerStop(ctx, containerName, nil); err != nil {
				fmt.Println(err)
			}

			if err := cli.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{}); err != nil {
				fmt.Println(err)
			}
		}
	}

	spinr.Stop()
	fmt.Println()

	if err := cli.NetworkRemove(ctx, fmt.Sprintf("net-%s", clusterName)); err != nil {
		fmt.Println(err)
	}

	return nil
}
