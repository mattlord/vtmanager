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

const (
	BACKUP_STORAGE_IMPL         = "file"
	BACKUP_ENGINE_IMPL          = "builtin"
	BACKUP_BASEDIR              = "/tmp"
	ETCD_CONTAINER_IMAGE        = "docker.io/bitnami/etcd:3.5.0"
	ETCD_PORT                   = 2379
	INITIAL_TABLET_ID           = "001"
	MYSQL_CONTAINER_IMAGE       = "docker.io/mysql/mysql-server"
	MYSQL_PASSWD                = ""
	MYSQL_PORT                  = 3306
	MYSQL_USER                  = "root"
	MYSQL_SOCKET                = "/tmp/mysql.sock"
	SINGLE_SHARD_RANGE          = "-"
	TOPO_GLOBAL_ROOT            = "/vitess/global"
	UNSHARDED_KS                = "unsharded_ks"
	UNSHARDED_DB                = "unsharded_db"
	VITESS_LITE_CONTAINER_IMAGE = "docker.io/vitess/lite"
	VITESS_CELL                 = "local"
	VT_GRPC_PORT                = 15999
	VT_MYSQL_PORT               = 15306
	VT_WEB_PORT                 = 15800
)

type vitessContainer struct {
	name       string
	image      string
	config     container.Config
	hostconfig container.HostConfig
	netconfig  network.NetworkingConfig
}

var (
	clusterVersion      string
	mysqlVersion        string
	topoGlobalRootFlag  = string(fmt.Sprintf("-topo_global_root=%s", TOPO_GLOBAL_ROOT))
	baseTabletAliasFlag = string(fmt.Sprintf("-tablet-path=%s", VITESS_CELL))
	cellFlag            = string(fmt.Sprintf("-cell=%s", VITESS_CELL))
	topoImplFlag        = string("-topo_implementation=etcd2")
	//mysqlStaticAuthStr  = string("{'mysql_user': [ { 'Password': '', 'UserData': 'root' } ] }")
	vitessContainers = []*vitessContainer{
		{
			name: "etcd",
			config: container.Config{
				Image: ETCD_CONTAINER_IMAGE,
				Env: []string{
					"ALLOW_NONE_AUTHENTICATION=yes",
					"ETCD_ENABLE_V2=true",
					"ETCDCTL_API=2",
					fmt.Sprintf("ETCD_ADVERTISE_CLIENT_URLS=http://0.0.0.0:%d", ETCD_PORT),
					fmt.Sprintf("ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:%d", ETCD_PORT),
				},
				Tty: false,
			},
			netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			name: "vtctld",
			config: container.Config{
				Image: VITESS_LITE_CONTAINER_IMAGE,
				Cmd: []string{
					"/vt/bin/vtctld",
					topoImplFlag,
					topoGlobalRootFlag,
					cellFlag,
					fmt.Sprintf("-backup_storage_implementation=%s", BACKUP_STORAGE_IMPL),
					fmt.Sprintf("-backup_engine_implementation=%s", BACKUP_ENGINE_IMPL),
					fmt.Sprintf("-file_backup_storage_root=%s", BACKUP_BASEDIR),
					"-service_map=grpc-vtctl,grpc-vtctld",
				},
				Tty: false,
			},
			netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			name: "mysqld",
			config: container.Config{
				Image: MYSQL_CONTAINER_IMAGE,
				Env: []string{
					fmt.Sprintf("MYSQL_DATABASE=%s", UNSHARDED_DB),
					"MYSQL_ALLOW_EMPTY_PASSWORD=yes",
					fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", MYSQL_PASSWD),
					"MYSQL_ROOT_HOST=%",
				},
				Tty: false,
			},
			netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			name: "vttablet",
			config: container.Config{
				Image: VITESS_LITE_CONTAINER_IMAGE,
				Cmd: []string{
					"/vt/bin/vttablet",
					topoImplFlag,
					topoGlobalRootFlag,
					"-enable_replication_reporter",
					fmt.Sprintf("%s-%s", baseTabletAliasFlag, INITIAL_TABLET_ID),
					"-init_tablet_type=replica",
					fmt.Sprintf("-init_keyspace=%s", UNSHARDED_KS),
					fmt.Sprintf("-init_shard=%s", SINGLE_SHARD_RANGE),
					fmt.Sprintf("-db_port=%d", MYSQL_PORT),
					fmt.Sprintf("-db_app_user=%s", MYSQL_USER),
					fmt.Sprintf("-db_app_password=%s", MYSQL_PASSWD),
					fmt.Sprintf("-db_dba_user=%s", MYSQL_USER),
					fmt.Sprintf("-db_dba_password=%s", MYSQL_PASSWD),
					fmt.Sprintf("-db_repl_user=%s", MYSQL_USER),
					fmt.Sprintf("-db_repl_password=%s", MYSQL_PASSWD),
					fmt.Sprintf("-db_filtered_user=%s", MYSQL_USER),
					fmt.Sprintf("-db_filtered_password=%s", MYSQL_PASSWD),
					fmt.Sprintf("-db_allprivs_user=%s", MYSQL_USER),
					fmt.Sprintf("-db_allprivs_password=%s", MYSQL_PASSWD),
					fmt.Sprintf("-init_db_name_override=%s", UNSHARDED_DB),
					fmt.Sprintf("-backup_storage_implementation=%s", BACKUP_STORAGE_IMPL),
					fmt.Sprintf("-backup_engine_implementation=%s", BACKUP_ENGINE_IMPL),
					fmt.Sprintf("-file_backup_storage_root=%s", BACKUP_BASEDIR),
					"-service_map=grpc-queryservice,grpc-tabletmanager,grpc-updatestream",
				},
				Tty: false,
			},
			netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			name: "vtgate",
			config: container.Config{
				Image: VITESS_LITE_CONTAINER_IMAGE,
				Cmd: []string{
					"/vt/bin/vtgate",
					topoImplFlag,
					topoGlobalRootFlag,
					cellFlag,
					"-tablet_types_to_wait=MASTER,REPLICA",
					fmt.Sprintf("-cells_to_watch=%s", VITESS_CELL),
					"-service_map=grpc-vtgateservice",
					fmt.Sprintf("-mysql_server_port=%d", VT_MYSQL_PORT),
					"-mysql_server_bind_address=0.0.0.0",
					fmt.Sprintf("-mysql_server_socket_path=%s", MYSQL_SOCKET),
					"-mysql_auth_server_impl=none",
					//"-mysql_auth_server_impl=static",
					//fmt.Sprintf("-mysql_auth_server_static_string=\"%s\"", mysqlStaticAuthStr),
				},
				Tty: false,
			},
			netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
	}
)

func runCreate(cmd *cobra.Command, args []string) {
	fmt.Printf("create called with %s\n", args)
}

func runCreateCluster(cmd *cobra.Command, args []string) {
	fmt.Printf("create cluster called with %s\n", args)

	clusterName := args[0]
	etcdAddr := fmt.Sprintf("http://etcd-%s:%d", clusterName, ETCD_PORT)
	networkName := fmt.Sprintf("net-%s", clusterName)
	topoGobalServerAddrFlag := fmt.Sprintf("-topo_global_server_address=%s", etcdAddr)
	vtctldServerFlag := fmt.Sprintf("-server=vtctld-%s:%d", clusterName, VT_GRPC_PORT)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	if _, err := cli.NetworkCreate(ctx, networkName, types.NetworkCreate{CheckDuplicate: true}); err != nil {
		fmt.Printf("Error when creating network %s: %s\n", networkName, err)
	}

	for _, vitessContainer := range vitessContainers {
		vitessContainer.config.Hostname = fmt.Sprintf("%s-%s", vitessContainer.name, clusterName)
		vitessContainer.hostconfig.RestartPolicy = container.RestartPolicy{"on-failure", 100}

		if vitessContainer.name == "mysqld" && mysqlVersion != "latest" {
			vitessContainer.config.Image += ":" + mysqlVersion
		}

		if strings.HasPrefix(vitessContainer.name, "vt") {
			addlVitessFlags := []string{
				topoGobalServerAddrFlag,
				fmt.Sprintf("-port=%d", VT_WEB_PORT),
				fmt.Sprintf("-grpc_port=%d", VT_GRPC_PORT),
			}
			vitessContainer.config.Cmd = append(vitessContainer.config.Cmd, addlVitessFlags...)

			if clusterVersion != "latest" {
				vitessContainer.config.Image += ":v" + clusterVersion
			}

			if vitessContainer.name == "vttablet" {
				addlTabletFlags := []string{
					fmt.Sprintf("-vtctld_addr=http://vtctld-%s:%d/", clusterName, VT_WEB_PORT),
					fmt.Sprintf("-db_host=mysqld-%s", clusterName),
					fmt.Sprintf("-tablet_hostname=%s", vitessContainer.config.Hostname),
				}
				vitessContainer.config.Cmd = append(vitessContainer.config.Cmd, addlTabletFlags...)
			}
		}

		reader, err := cli.ImagePull(ctx, vitessContainer.config.Image, types.ImagePullOptions{})
		if err != nil {
			fmt.Printf("Container image %s not found\n", vitessContainer.image)
			os.Exit(1)
		}
		io.Copy(os.Stdout, reader)

		vitessContainer.netconfig.EndpointsConfig["NetworkID"] = &network.EndpointSettings{
			NetworkID: networkName,
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
		if vitessContainer.name == "mysqld" && resp.ID != "" {
			spinr := spinner.New(spinner.CharSets[9], 150*time.Millisecond)

			fmt.Println("Waiting for mysqld to initialize and be ready to serve queries ...")
			spinr.Start()
			time.Sleep(60 * time.Second) // we know it typically takes 60+ seconds
			for retryCount := 0; retryCount < execRetries; retryCount++ {
				if execExitCode = util.ContainerExec(ctx, cli, resp.ID, []string{"mysql", "-u", "root", "-BNe", "'select 1'"}); execExitCode == 0 {
					break
				}
				time.Sleep(10 * time.Second)
			}
			spinr.Stop()

			if execExitCode != 0 {
				panic(fmt.Sprintf("mysqld did not become ready to serve queries after %d retries, giving up!\n", execRetries))
			}
		}

		// We need to create the cell
		if vitessContainer.name == "vtctld" && resp.ID != "" {
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
		if vitessContainer.name == "vttablet" && resp.ID != "" {
			keyspaceShard := fmt.Sprintf("%s/%s", UNSHARDED_KS, SINGLE_SHARD_RANGE)
			tabletAlias := fmt.Sprintf("%s-%s", VITESS_CELL, INITIAL_TABLET_ID)
			tabletStatusURL := fmt.Sprintf("http://vttablet-%s:%d/debug/status", clusterName, VT_WEB_PORT)

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
