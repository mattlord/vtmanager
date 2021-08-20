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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const (
	EXEC_CREATE_FAILED  = 100
	EXEC_ATTACH_FAILED  = 101
	EXEC_START_FAILED   = 102
	EXEC_INSPECT_FAILED = 103
)

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
