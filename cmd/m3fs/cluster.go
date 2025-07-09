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
	"encoding/json"
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
	"github.com/open3fs/m3fs/pkg/utils"
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
			Name:   "add-client",
			Usage:  "Add 3fs cluster clients",
			Action: addClient,
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
		&fsclient.Create3FSClientServiceTask{
			ClientNodes: cfg.Services.Client.Nodes,
		},
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

func syncClientModels(cfg *config.Config, db *gorm.DB) error {
	var nodes []model.Node
	err := db.Model(new(model.Node)).Find(&nodes).Error
	if err != nil {
		return errors.Trace(err)
	}
	nodesMap := make(map[string]model.Node, len(nodes))
	for _, node := range nodes {
		nodesMap[node.Name] = node
	}

	var clients []model.FuseClient
	err = db.Model(new(model.FuseClient)).Find(&clients).Error
	if err != nil {
		return errors.Trace(err)
	}
	clientsMap := make(map[uint]model.FuseClient, len(clients))
	for _, client := range clients {
		clientsMap[client.NodeID] = client
	}

	clientNodes := cfg.Services.Client.Nodes
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, clientNode := range clientNodes {
			node, ok := nodesMap[clientNode]
			if !ok {
				return errors.Errorf("client node %s not found", clientNode)
			}
			_, ok = clientsMap[node.ID]
			if ok {
				continue
			}
			client := model.FuseClient{
				Name:           cfg.Services.Client.ContainerName,
				NodeID:         node.ID,
				HostMountpoint: cfg.Services.Client.HostMountpoint,
			}
			if err = tx.Model(&client).Create(&client).Error; err != nil {
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
	err = query.Find(&newStorNodes, "name IN (?)", cfg.Services.Storage.Nodes).Error
	if err != nil {
		return errors.Trace(err)
	}
	if len(newStorNodes) == 0 && changePlan == nil {
		return errors.New("No new storage nodes to add")
	}
	storCreated := false
	for _, step := range steps {
		if step.OperationType == model.ChangePlanStepOpType.CreateStorService && step.FinishAt != nil {
			storCreated = true
			break
		}
	}
	if !storCreated && changePlan != nil {
		var data model.ChangePlanAddStorNodesData
		if err = json.Unmarshal([]byte(changePlan.Data), &data); err != nil {
			return errors.Annotate(err, "parse change plan data")
		}
		if err = db.Model(new(model.Node)).
			Find(&newStorNodes, "id IN (?)", data.NewNodeIDs).Error; err != nil {

			return errors.Trace(err)
		}
	}
	// NOTE: if change plan is not created yet or create storage service step not finished, do create
	//  storage service task again with new storage nodes.
	if !storCreated {
		newNodeNames := make([]string, len(newStorNodes))
		for i, node := range newStorNodes {
			newNodeNames[i] = node.Name
			nodesMap[node.Name] = node
		}
		log.Logger.Infof("Add new storage nodes: %v", newNodeNames)
		tasks = append(tasks, &storage.CreateStorageServiceTask{
			DeleteContainerIfExists: true,
			StorageNodes:            newNodeNames,
			BeginNodeID:             10001 + len(storNodeIDs),
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
		return errors.Annotate(err, "add storage nodes")
	}

	log.Logger.Infof("Add storage nodes success")

	return nil
}

func addClient(ctx *cli.Context) error {
	cfg, err := loadClusterConfig()
	if err != nil {
		return errors.Trace(err)
	}

	db, err := setupDB(cfg)
	if err != nil {
		return errors.Trace(err)
	}

	nodes := []*model.Node{}
	err = db.Model(new(model.Node)).Find(&nodes).Error
	if err != nil {
		return errors.Trace(err)
	}
	nodesMap := make(map[string]*model.Node, len(nodes))
	for _, node := range nodes {
		nodesMap[node.Name] = node
	}

	clientNodes := []string{}
	err = db.Raw(
		"SELECT nodes.name FROM fuse_clients INNER JOIN nodes ON fuse_clients.node_id = nodes.id",
	).Scan(&clientNodes).Error
	if err != nil {
		return errors.Trace(err)
	}
	clientNodeSet := utils.NewSet(clientNodes...)
	newClientNodes := []string{}
	for _, clientNode := range cfg.Services.Client.Nodes {
		if !clientNodeSet.Contains(clientNode) {
			newClientNodes = append(newClientNodes, clientNode)
		}
	}

	if err = syncNodeModels(cfg, db); err != nil {
		return errors.Trace(err)
	}
	if len(newClientNodes) == 0 {
		return errors.New("No new client nodes to add")
	}

	runner, err := task.NewRunner(cfg,
		&fsclient.Create3FSClientServiceTask{
			ClientNodes:             newClientNodes,
			DeleteContainerIfExists: true,
		},
	)
	if err != nil {
		return errors.Trace(err)
	}
	runner.Init()
	runner.Runtime.Store(task.RuntimeDbKey, db)
	runner.Runtime.Store(task.RuntimeNodesMapKey, nodesMap)

	if err = runner.Run(ctx.Context); err != nil {
		return errors.Annotate(err, "add client")
	}

	if err = syncClientModels(cfg, db); err != nil {
		return errors.Trace(err)
	}

	log.Logger.Infof("Add client success")

	return nil
}
