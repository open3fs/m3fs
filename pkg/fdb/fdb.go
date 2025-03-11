package fdb

import (
	"context"
	"fmt"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/image"
	"github.com/open3fs/m3fs/pkg/task"
)

type genClusterFileContentStep struct {
	task.BaseStep
}

func (s *genClusterFileContentStep) Execute(context.Context) error {
	nodes := make([]string, len(s.Runtime.Services.Fdb.Nodes))
	fdb := s.Runtime.Services.Fdb
	for i, fdbNode := range fdb.Nodes {
		for _, node := range s.Runtime.Nodes {
			if node.Name == fdbNode {
				nodes[i] = fmt.Sprintf("%s:%s@%s", node.Name, node.Name,
					net.JoinHostPort(node.Host, strconv.Itoa(fdb.Port)))
			}
		}
	}

	s.Logger.Infof("fdb cluster file content: %s", strings.Join(nodes, ","))
	s.Runtime.Store("fdb_cluster_file_content", strings.Join(nodes, ","))
	return nil
}

type startContainerStep struct {
	task.BaseStep
}

func (s *startContainerStep) Execute(ctx context.Context) error {
	dataDir := path.Join(s.Runtime.Services.Fdb.DataDir, "data")
	_, err := s.Em.OS.Exec(ctx, "mkdir", "-p", dataDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", dataDir)
	}
	logDir := path.Join(s.Runtime.Services.Fdb.DataDir, "log")
	_, err = s.Em.OS.Exec(ctx, "mkdir", "-p", logDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", dataDir)
	}
	img, err := image.GetImage(s.Runtime.Cfg.Registry.CustomRegistry, "fdb")
	if err != nil {
		return errors.Trace(err)
	}
	clusterContentI, _ := s.Runtime.Load("fdb_cluster_file_content")
	clusterContent := clusterContentI.(string)
	args := &external.RunArgs{
		Image:       img,
		Name:        common.Pointer("fdb"),
		HostNetwork: true,
		Envs: map[string]string{
			"FDB_CLUSTER_FILE_CONTENTS": clusterContent,
		},
		Volumes: []*external.VolumeArgs{
			{
				Source: dataDir,
				Target: "/var/fdb/data",
			},
			{
				Source: logDir,
				Target: "/var/fdb/log",
			},
		},
	}
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	s.Logger.Infof("Start fdb container success")
	return nil
}

type initClusterStep struct {
	task.BaseStep
}

func (s *initClusterStep) Execute(ctx context.Context) error {
	err := s.initCluster(ctx)
	if err != nil {
		return errors.Trace(err)
	}

	return s.waitClusterInitilized(ctx)
}

func (s *initClusterStep) initCluster(ctx context.Context) error {
	s.Logger.Infof("Initialize fdb cluster")
	// TODO: initialize fdb cluster with replication and coordinator setting
	_, err := s.Em.OS.Exec(ctx, "docker", "exec", "fdbcli", "--exec", "configure single ssd")
	if err != nil {
		return errors.Annotate(err, "initialize fdb cluster")
	}

	return nil
}

func (s *initClusterStep) waitClusterInitilized(ctx context.Context) error {
	s.Logger.Infof("Waiting for fdb cluster initialized")
	tctx, cancel := context.WithTimeout(ctx, s.Runtime.Services.Fdb.WaitClusterTimeout)
	defer cancel()

	for {
		out, err := s.Em.OS.Exec(tctx, "docker", "exec", "fdbcli", "--exec", "status minimal")
		if err != nil {
			return errors.Annotate(err, "wait fdb cluster initialized")
		}
		if out == "The database is available." {
			break
		}
		time.Sleep(time.Second)
	}

	s.Logger.Infof("Fdb cluster initialized")
	return nil
}

// CreateFdbClusterTask is a task for creating a new FoundationDB cluster.
type CreateFdbClusterTask struct {
	task.BaseTask
}

// Init initializes the task.
func (t *CreateFdbClusterTask) Init(r *task.Runtime) {
	t.BaseTask.Init(r)
	t.BaseTask.SetName("CreateFdbClusterTask")
	nodes := make([]config.Node, len(r.Cfg.Services.Fdb.Nodes))
	for i, node := range r.Cfg.Services.Fdb.Nodes {
		nodes[i] = r.Nodes[node]
	}
	t.SetSteps([]task.StepConfig{
		{
			Nodes:   []config.Node{nodes[0]},
			NewStep: func() task.Step { return new(genClusterFileContentStep) },
		},
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(startContainerStep) },
		},
		{
			Nodes:   nodes,
			NewStep: func() task.Step { return new(initClusterStep) },
		},
	})
}
