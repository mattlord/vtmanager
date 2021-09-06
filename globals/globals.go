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
package globals

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
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
	SPINNER_CHARSET             = 14
	TOPO_GLOBAL_ROOT            = "/vitess/global"
	UNSHARDED_KS                = "unsharded_ks"
	UNSHARDED_DB                = "unsharded_db"
	VITESS_LITE_CONTAINER_IMAGE = "docker.io/vitess/lite"
	VITESS_CELL                 = "local"
	VT_GRPC_PORT                = 15999
	VT_MYSQL_PORT               = 15306
	VT_WEB_PORT                 = 15800
)

type VitessContainer struct {
	Name       string
	Count      int
	Config     container.Config
	Hostconfig container.HostConfig
	Netconfig  network.NetworkingConfig
}

var (
	ClusterVersion      string
	MysqlVersion        string
	TopoGlobalRootFlag  = string(fmt.Sprintf("-topo_global_root=%s", TOPO_GLOBAL_ROOT))
	BaseTabletAliasFlag = string(fmt.Sprintf("-tablet-path=%s", VITESS_CELL))
	CellFlag            = string(fmt.Sprintf("-cell=%s", VITESS_CELL))
	TopoImplFlag        = string("-topo_implementation=etcd2")
	//mysqlStaticAuthStr  = string("{'mysql_user': [ { 'Password': '', 'UserData': 'root' } ] }")
	VitessContainers = []*VitessContainer{
		{
			Name:  "etcd",
			Count: 1,
			Config: container.Config{
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
			Netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			Name:  "vtctld",
			Count: 1,
			Config: container.Config{
				Image: VITESS_LITE_CONTAINER_IMAGE,
				Cmd: []string{
					"/vt/bin/vtctld",
					TopoImplFlag,
					TopoGlobalRootFlag,
					CellFlag,
					fmt.Sprintf("-backup_storage_implementation=%s", BACKUP_STORAGE_IMPL),
					fmt.Sprintf("-backup_engine_implementation=%s", BACKUP_ENGINE_IMPL),
					fmt.Sprintf("-file_backup_storage_root=%s", BACKUP_BASEDIR),
					"-service_map=grpc-vtctl,grpc-vtctld",
				},
				Tty: false,
			},
			Netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			Name:  "mysqld",
			Count: 2,
			Config: container.Config{
				Image: MYSQL_CONTAINER_IMAGE,
				Cmd: []string{
					"--gtid-mode=ON",
					"--enforce_gtid_consistency=ON",
				},
				Env: []string{
					"MYSQL_ALLOW_EMPTY_PASSWORD=yes",
					fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", MYSQL_PASSWD),
					"MYSQL_ROOT_HOST=%",
				},
				Tty: false,
			},
			Netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			Name:  "vttablet",
			Count: 2,
			Config: container.Config{
				Image: VITESS_LITE_CONTAINER_IMAGE,
				Cmd: []string{
					"/vt/bin/vttablet",
					TopoImplFlag,
					TopoGlobalRootFlag,
					"-enable_replication_reporter",
					"-init_tablet_type=replica",
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
					fmt.Sprintf("-backup_storage_implementation=%s", BACKUP_STORAGE_IMPL),
					fmt.Sprintf("-backup_engine_implementation=%s", BACKUP_ENGINE_IMPL),
					fmt.Sprintf("-file_backup_storage_root=%s", BACKUP_BASEDIR),
					"-service_map=grpc-queryservice,grpc-tabletmanager,grpc-updatestream",
				},
				Tty: false,
			},
			Netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
		{
			Name:  "vtgate",
			Count: 1,
			Config: container.Config{
				Image: VITESS_LITE_CONTAINER_IMAGE,
				Cmd: []string{
					"/vt/bin/vtgate",
					TopoImplFlag,
					TopoGlobalRootFlag,
					CellFlag,
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
			Netconfig: network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			},
		},
	}
)
