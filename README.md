# vtmanager
Tool for managing container based Vitess deployments for testing purposes

# Install
go get github.com/mattlord/vtmanager

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
