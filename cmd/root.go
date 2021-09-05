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
	"fmt"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	BACKUP_STORAGE_IMPL         = "file"
	BACKUP_ENGINE_IMPL          = "builtin"
	BACKUP_BASEDIR              = "/tmp"
	ETCD_CONTAINER_IMAGE        = "docker.io/bitnami/etcd:3.5.0"
	ETCD_PORT                   = 2379
	MYSQL_CONTAINER_IMAGE       = "docker.io/mysql/mysql-server"
	MYSQL_PASSWD                = ""
	MYSQL_PORT                  = 3306
	MYSQL_USER                  = "root"
	MYSQL_SOCKET                = "/tmp/mysql.sock"
	SINGLE_SHARD_RANGE          = "-"
	SPANNER_CHARSET             = 14
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
	count      int
	config     container.Config
	hostconfig container.HostConfig
	netconfig  network.NetworkingConfig
}

var (
	cfgFile             string
	clusterName         string
	clusterVersion      string
	mysqlVersion        string
	topoGlobalRootFlag  = string(fmt.Sprintf("-topo_global_root=%s", TOPO_GLOBAL_ROOT))
	baseTabletAliasFlag = string(fmt.Sprintf("-tablet-path=%s", VITESS_CELL))
	cellFlag            = string(fmt.Sprintf("-cell=%s", VITESS_CELL))
	topoImplFlag        = string("-topo_implementation=etcd2")
	//mysqlStaticAuthStr  = string("{'mysql_user': [ { 'Password': '', 'UserData': 'root' } ] }")
	vitessContainers = []*vitessContainer{
		{
			name:  "etcd",
			count: 1,
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
			name:  "vtctld",
			count: 1,
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
			name:  "mysqld",
			count: 2,
			config: container.Config{
				Image: MYSQL_CONTAINER_IMAGE,
				Cmd: []string{
					"--gtid-mode=ON",
					"--enforce_gtid_consistency=ON",
				},
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
			name:  "vttablet",
			count: 2,
			config: container.Config{
				Image: VITESS_LITE_CONTAINER_IMAGE,
				Cmd: []string{
					"/vt/bin/vttablet",
					topoImplFlag,
					topoGlobalRootFlag,
					"-enable_replication_reporter",
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
			name:  "vtgate",
			count: 1,
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

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "vtmanager",
	Short: "Manage local containerized Vitess deployments",
	Long:  `Manage local containerized Vitess deployments`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.vtmanager.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".vtmanager" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".vtmanager")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
