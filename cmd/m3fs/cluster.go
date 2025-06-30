// Copyright 2025 Open3FS Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	fsclient "github.com/open3fs/m3fs/pkg/3fs_client"
	"github.com/open3fs/m3fs/pkg/artifact"
	"github.com/open3fs/m3fs/pkg/clickhouse"
	"github.com/open3fs/m3fs/pkg/config"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/fdb"
	"github.com/open3fs/m3fs/pkg/grafana"
	"github.com/open3fs/m3fs/pkg/log"
	"github.com/open3fs/m3fs/pkg/meta"
	"github.com/open3fs/m3fs/pkg/mgmtd"
	"github.com/open3fs/m3fs/pkg/monitor"
	"github.com/open3fs/m3fs/pkg/network"
	"github.com/open3fs/m3fs/pkg/pg"
	"github.com/open3fs/m3fs/pkg/pg/model"
	"github.com/open3fs/m3fs/pkg/storage"
	"github.com/open3fs/m3fs/pkg/task"
)

var clusterCmd = &cli.Command{
	Name:    "cluster",
	Aliases: []string{"c"},
	Usage:   "Manage 3fs cluster",
	Subcommands: []*cli.Command{
		{
			Name:   "create",
			Usage:  "Create a new 3fs cluster",
			Action: createCluster,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "config",
					Aliases:     []string{"c"},
					Usage:       "Path to the cluster configuration file",
					Destination: &configFilePath,
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "workdir",
					Aliases:     []string{"w"},
					Usage:       "Path to the working directory (default is current directory)",
					Destination: &workDir,
				},
				&cli.StringFlag{
					Name:        "registry",
					Aliases:     []string{"r"},
					Usage:       "Image registry (default is empty)",
					Destination: &registry,
				},
			},
		},
		{
			Name:    "delete",
			Aliases: []string{"destroy"},
			Usage:   "Destroy a 3fs cluster",
			Action:  deleteCluster,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "config",
					Aliases:     []string{"c"},
					Usage:       "Path to the cluster configuration file",
					Destination: &configFilePath,
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "workdir",
					Aliases:     []string{"w"},
					Usage:       "Path to the working directory (default is current directory)",
					Destination: &workDir,
				},
				&cli.BoolFlag{
					Name:        "all",
					Aliases:     []string{"a"},
					Usage:       "Remove images, packages and scripts",
					Destination: &clusterDeleteAll,
				},
			},
		},
		{
			Name:   "add-storage-nodes",
			Usage:  "Add 3fs cluster storage nodes",
			Action: addStorageNodes,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "config",
					Aliases:     []string{"c"},
					Usage:       "Path to the cluster configuration file",
					Destination: &configFilePath,
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "workdir",
					Aliases:     []string{"w"},
					Usage:       "Path to the working directory (default is current directory)",
					Destination: &workDir,
				},
			},
		},
		{
			Name:   "prepare",
			Usage:  "Prepare to deploy a 3fs cluster",
			Action: prepareCluster,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "config",
					Aliases:     []string{"c"},
					Usage:       "Path to the cluster configuration file",
					Destination: &configFilePath,
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "artifact",
					Aliases:     []string{"a"},
					Usage:       "Path to the 3fs artifact",
					Destination: &artifactPath,
					Required:    false,
				},
			},
		},
		{
			Name:    "architecture",
			Aliases: []string{"arch"},
			Usage:   "Generate architecture diagram of a 3fs cluster",
			Action:  drawClusterArchitecture,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:        "config",
					Aliases:     []string{"c"},
					Usage:       "Path to the cluster configuration file",
					Destination: &configFilePath,
					Required:    true,
				},
				&cli.BoolFlag{
					Name:        "no-color",
					Usage:       "Disable colored output in the diagram",
					Destination: &noColorOutput,
				},
			},
		},
	},
}

func loadClusterConfig() (*config.Config, error) {
	cfg := config.NewConfigWithDefaults()
	file, err := os.Open(configFilePath)
	if err != nil {
		return nil, errors.Annotate(err, "open config file")
	}
	if err = yaml.NewDecoder(file).Decode(cfg); err != nil {
		return nil, errors.Annotate(err, "load cluster config")
	}
	if err = cfg.SetValidate(workDir, registry); err != nil {
		return nil, errors.Annotate(err, "validate cluster config")
	}
	log.Logger.Debugf("Cluster config: %+v", cfg)

	return cfg, nil
}

func createCluster(ctx *cli.Context) error {
	cfg, err := loadClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}

	runner, err := task.NewRunner(cfg,
		new(pg.CreatePgClusterTask),
		new(fdb.CreateFdbClusterTask),
		new(clickhouse.CreateClickhouseClusterTask),
		new(grafana.CreateGrafanaServiceTask),
		new(monitor.CreateMonitorTask),
		new(mgmtd.CreateMgmtdServiceTask),
		new(meta.CreateMetaServiceTask),
		&storage.CreateStorageServiceTask{
			StorageNodes: cfg.Services.Storage.Nodes,
			BeginNodeID:  10001,
		},
		new(mgmtd.InitUserAndChainTask),
		new(fsclient.Create3FSClientServiceTask),
	)
	if err != nil {
		return errors.Trace(err)
	}
	runner.Init()
	if err = runner.Run(ctx.Context); err != nil {
		return errors.Annotate(err, "create cluster")
	}
	log.Logger.Infof("3FS is mounted at %s on node %s",
		cfg.Services.Client.HostMountpoint, strings.Join(cfg.Services.Client.Nodes, ","))

	return nil
}

func deleteCluster(ctx *cli.Context) error {
	cfg, err := loadClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}

	runnerTasks := []task.Interface{
		new(fsclient.Delete3FSClientServiceTask),
		new(storage.DeleteStorageServiceTask),
		new(meta.DeleteMetaServiceTask),
		new(mgmtd.DeleteMgmtdServiceTask),
		new(monitor.DeleteMonitorTask),
		new(grafana.DeleteGrafanaServiceTask),
		new(clickhouse.DeleteClickhouseClusterTask),
		new(fdb.DeleteFdbClusterTask),
		new(pg.DeletePgClusterTask),
	}
	if clusterDeleteAll {
		runnerTasks = append(runnerTasks, new(network.DeleteNetworkTask))
	}
	runner, err := task.NewRunner(cfg, runnerTasks...)
	if err != nil {
		return errors.Trace(err)
	}
	runner.Init()
	if err = runner.Run(ctx.Context); err != nil {
		return errors.Annotate(err, "delete cluster")
	}

	return nil
}

func prepareCluster(ctx *cli.Context) error {
	cfg, err := loadClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}
	runnerTasks := []task.Interface{}
	if artifactPath != "" {
		runnerTasks = append(runnerTasks, new(artifact.ImportArtifactTask))
	}
	runnerTasks = append(runnerTasks, new(network.PrepareNetworkTask))

	runner, err := task.NewRunner(cfg, runnerTasks...)
	if err != nil {
		return errors.Trace(err)
	}
	runner.Init()
	if artifactPath != "" {
		if err = runner.Store(task.RuntimeArtifactPathKey, artifactPath); err != nil {
			return errors.Trace(err)
		}
	}
	if err = runner.Run(ctx.Context); err != nil {
		return errors.Annotate(err, "prepare cluster")
	}

	return nil
}

func drawClusterArchitecture(ctx *cli.Context) error {
	cfg, err := loadClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}

	diagram, err := NewArchDiagram(cfg, noColorOutput)
	if err != nil {
		return errors.Trace(err)
	}

	fmt.Println(diagram.Render())
	return nil
}

func setupDB(cfg *config.Config) (*gorm.DB, error) {
	dbCfg := cfg.Services.Pg
	dbNode := cfg.Services.Pg.Nodes[0]
	for _, node := range cfg.Nodes {
		if node.Name != dbNode {
			continue
		}

		db, err := model.NewDB(&model.ConnectionArgs{
			Host:             node.Host,
			Port:             dbCfg.Port,
			User:             dbCfg.Username,
			Password:         dbCfg.Password,
			DBName:           dbCfg.Database,
			SlowSQLThreshold: 200 * time.Millisecond,
		})
		if err != nil {
			return nil, errors.Trace(err)
		}
		return db, nil
	}
	return nil, errors.New("no db nodes found")
}

func syncNodeModels(cfg *config.Config, db *gorm.DB) error {
	var nodes []model.Node
	err := db.Model(new(model.Node)).Find(&nodes).Error
	if err != nil {
		return errors.Trace(err)
	}
	nodesMap := make(map[string]model.Node, len(nodes))
	for _, node := range nodes {
		nodesMap[node.Name] = node
	}
	cfgNodes := cfg.Nodes
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, cfgNode := range cfgNodes {
			_, ok := nodesMap[cfgNode.Name]
			if ok {
				continue
			}
			node := model.Node{
				Name: cfgNode.Name,
				Host: cfgNode.Host,
			}
			if err = tx.Model(&node).Create(&node).Error; err != nil {
				return errors.Trace(err)
			}
		}

		return nil
	})
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func addStorageNodes(ctx *cli.Context) error {
	cfg, err := loadClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}

	db, err := setupDB(cfg)
	if err != nil {
		return errors.Trace(err)
	}

	if err = syncNodeModels(cfg, db); err != nil {
		return errors.Trace(err)
	}

	changePlan, err := model.GetProcessingChangePlan(db)
	if err != nil {
		return errors.Trace(err)
	}
	var steps []*model.ChangePlanStep
	if changePlan != nil {
		if changePlan.Type != model.ChangePlanTypeAddStorNodes {
			return errors.Errorf("there are unfinished %s operation", changePlan.Type)
		}
		steps, err = changePlan.GetSteps(db)
		if err != nil {
			return errors.Trace(err)
		}
	}

	var tasks []task.Interface
	if changePlan == nil {
		tasks = append(tasks, new(storage.PrepareChangePlanTask))
	}
	nodesMap := make(map[string]*model.Node)
	var storNodeIDs []uint
	err = db.Model(new(model.StorService)).Select("node_id").Find(&storNodeIDs).Error
	if err != nil {
		return errors.Trace(err)
	}
	var newStorNodes []*model.Node
	query := db.Model(new(model.Node))
	if len(storNodeIDs) != 0 {
		query = query.Where("id NOT IN (?)", storNodeIDs)
	}
	err = query.Find(&newStorNodes, "name in (?)", cfg.Services.Storage.Nodes).Error
	if err != nil {
		return errors.Trace(err)
	}
	if len(newStorNodes) == 0 && changePlan == nil {
		return errors.New("No new storage nodes to add")
	}
	if changePlan == nil {
		newNodeNames := make([]string, len(newStorNodes))
		for i, node := range newStorNodes {
			newNodeNames[i] = node.Name
			nodesMap[node.Name] = node
		}
		log.Logger.Infof("Add new storage nodes: %v", newNodeNames)
		tasks = append(tasks, &storage.CreateStorageServiceTask{
			StorageNodes: newNodeNames,
			BeginNodeID:  10001 + len(storNodeIDs),
		})
	}
	tasks = append(tasks, new(storage.RunChangePlanTask))

	runner, err := task.NewRunner(cfg, tasks...)
	if err != nil {
		return errors.Trace(err)
	}
	runner.Init()
	runner.Runtime.Store(task.RuntimeDbKey, db)
	runner.Runtime.Store(task.RuntimeChangePlanKey, changePlan)
	runner.Runtime.Store(task.RuntimeChangePlanStepsKey, steps)
	runner.Runtime.Store(task.RuntimeNodesMapKey, nodesMap)

	if err = runner.Run(ctx.Context); err != nil {
		return errors.Annotate(err, "create cluster")
	}

	log.Logger.Infof("Add storage nodes success")

	return nil
}
