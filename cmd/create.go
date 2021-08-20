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

	"github.com/spf13/cobra"
)

func runCreate(cmd *cobra.Command, args []string) {
	fmt.Printf("create called with %s\n", args)
}

func runCreateCluster(cmd *cobra.Command, args []string) {
	fmt.Printf("create cluster called with %s\n", args)
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

var subCreateClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Create Vitess cluster",
	Long:  ``,
	Run:   runCreateCluster,
}

var subCreateKeyspaceCmd = &cobra.Command{
	Use:   "keyspace",
	Short: "Create Vitess keyspace",
	Long:  ``,
	Run:   runCreateKeyspace,
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.AddCommand(subCreateClusterCmd)
	createCmd.AddCommand(subCreateKeyspaceCmd)

	createCmd.Flags().String("name", "", "object name")
}
