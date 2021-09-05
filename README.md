# vtmanager
Tool for managing container based Vitess deployments for testing purposes

# Install
```
go get github.com/mattlord/vtmanager
```

# Usage
```
$ vtmanager -h
Manage local containerized Vitess deployments

Usage:
  vtmanager [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  create      Create Vitess objects
  delete      Delete Vitess objects
  describe    Describe Vitess objects
  help        Help about any command
  list        List Vitess objects
  update      Update Vitess objects

Flags:
      --config string   config file (default is $HOME/.vtmanager.yaml)
  -h, --help            help for vtmanager
  -t, --toggle          Help message for toggle

Use "vtmanager [command] --help" for more information about a command.`
```

# Example
```
$ vtmanager create cluster test -v 11.0.0 -m 8.0
Pulling up to date container images
⠏ done!
Waiting for mysqld on mysqld-test-1 to initialize and be ready to serve queries
⠼ done!
Waiting for mysqld on mysqld-test-2 to initialize and be ready to serve queries
⠋ done!

$ docker ps -a
CONTAINER ID   IMAGE                    COMMAND                  CREATED              STATUS                   PORTS                       NAMES
f3660205f351   vitess/lite:v11.0.0      "/vt/bin/vtgate -top…"   About a minute ago   Up About a minute                                    vtgate-test
23bbb86922eb   vitess/lite:v11.0.0      "/vt/bin/vttablet -t…"   About a minute ago   Up About a minute                                    vttablet-test-2
86cbeb0fa302   vitess/lite:v11.0.0      "/vt/bin/vttablet -t…"   About a minute ago   Up About a minute                                    vttablet-test-1
58c83bec0554   mysql/mysql-server:8.0   "/entrypoint.sh --gt…"   3 minutes ago        Up 3 minutes (healthy)   3306/tcp, 33060-33061/tcp   mysqld-test-2
fcaa1c6afdaf   mysql/mysql-server:8.0   "/entrypoint.sh --gt…"   4 minutes ago        Up 4 minutes (healthy)   3306/tcp, 33060-33061/tcp   mysqld-test-1
60ce63ac4314   vitess/lite:v11.0.0      "/vt/bin/vtctld -top…"   4 minutes ago        Up 4 minutes                                         vtctld-test
604d4e51ef73   bitnami/etcd:3.5.0       "/opt/bitnami/script…"   4 minutes ago        Up 4 minutes             2379-2380/tcp               etcd-test

$ docker exec -it vttablet-test-1 /vt/bin/vtctlclient -server=vtctld-test:15999 ListAllTablets
local-0000000001 unsharded_ks - replica vttablet-test-1:15800 mysqld-test-1:3306 [] <null>
local-0000000002 unsharded_ks - master vttablet-test-2:15800 mysqld-test-2:3306 [] 2021-09-05T19:54:35Z

$ docker exec -it mysqld-test-1 mysql -u root -h vtgate-test -P 15306 unsharded_ks -e "create table t1 (id int)"

$ docker exec -it mysqld-test-1 mysql -u root -h vtgate-test -P 15306 unsharded_ks -e "show tables"
+------------------------+
| Tables_in_unsharded_ks |
+------------------------+
| t1                     |
+------------------------+

$ vtmanager delete cluster test
Stopping and deleting containers for the test cluster
⠸ done!

$ docker ps -a
CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES
```