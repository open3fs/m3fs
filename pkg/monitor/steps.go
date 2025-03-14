package monitor

import (
	"context"
	"embed"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"text/template"

	"github.com/open3fs/m3fs/pkg/common"
	"github.com/open3fs/m3fs/pkg/errors"
	"github.com/open3fs/m3fs/pkg/external"
	"github.com/open3fs/m3fs/pkg/image"
	"github.com/open3fs/m3fs/pkg/task"
)

var (
	//go:embed templates/*
	templatesFs embed.FS

	// MonitorCollectorMainTmpl is the template content of monitor_collector_main.toml
	MonitorCollectorMainTmpl []byte
)

func init() {
	var err error
	MonitorCollectorMainTmpl, err = templatesFs.ReadFile("templates/monitor_collector_main.tmpl")
	if err != nil {
		panic(err)
	}
}

type genMonitorConfigStep struct {
	task.BaseStep
}

func (s *genMonitorConfigStep) Execute(context.Context) error {
	tempDir, err := os.MkdirTemp(os.TempDir(), "3fs-monitor.")
	if err != nil {
		return errors.Trace(err)
	}
	s.Runtime.Store("monitor_temp_config_dir", tempDir)

	fileName := "monitor_collector_main.toml"
	tmpl, err := template.New(fileName).Parse(string(MonitorCollectorMainTmpl))
	if err != nil {
		return errors.Annotate(err, "parse monitor_collector_main.toml template")
	}
	configPath := filepath.Join(tempDir, fileName)
	file, err := os.Create(configPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			s.Logger.Warnf("Failed to close file %+v", err)
		}
	}()
	var clickhouseHost string
	for _, clickhouseNode := range s.Runtime.Services.Clickhouse.Nodes {
		for _, node := range s.Runtime.Nodes {
			if node.Name == clickhouseNode {
				clickhouseHost = node.Host
			}
		}
	}
	err = tmpl.Execute(file, map[string]string{
		"Db":       s.Runtime.Services.Clickhouse.Db,
		"Host":     clickhouseHost,
		"Password": s.Runtime.Services.Clickhouse.Password,
		"Port":     strconv.Itoa(s.Runtime.Services.Clickhouse.TCPPort),
		"User":     s.Runtime.Services.Clickhouse.User,
	})
	if err != nil {
		return errors.Annotate(err, "write monitor_collector_main.toml")
	}

	return nil
}

type runContainerStep struct {
	task.BaseStep
}

func (s *runContainerStep) Execute(ctx context.Context) error {
	etcDir := path.Join(s.Runtime.Services.Monitor.WorkDir, "etc")
	_, err := s.Em.Runner.Exec(ctx, "mkdir", "-p", etcDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", etcDir)
	}
	localConfigDir, _ := s.Runtime.Load("monitor_temp_config_dir")
	localConfigFile := path.Join(localConfigDir.(string), "monitor_collector_main.toml")
	remoteConfigFile := path.Join(etcDir, "monitor_collector_main.toml")
	if err := s.Em.Runner.Scp(localConfigFile, remoteConfigFile); err != nil {
		return errors.Annotatef(err, "scp monitor_collector_main.toml")
	}
	logDir := path.Join(s.Runtime.Services.Monitor.WorkDir, "log")
	_, err = s.Em.Runner.Exec(ctx, "mkdir", "-p", logDir)
	if err != nil {
		return errors.Annotatef(err, "mkdir %s", logDir)
	}

	img, err := image.GetImage(s.Runtime.Cfg.Registry.CustomRegistry, "3fs")
	if err != nil {
		return errors.Trace(err)
	}
	args := &external.RunArgs{
		Image:       img,
		Name:        &s.Runtime.Services.Monitor.ContainerName,
		HostNetwork: true,
		Privileged:  common.Pointer(true),
		Detach:      common.Pointer(true),
		Volumes: []*external.VolumeArgs{
			{
				Source: etcDir,
				Target: "/opt/3fs/etc",
			},
			{
				Source: logDir,
				Target: "/var/log/3fs",
			},
		},
		Command: []string{
			"/opt/3fs/bin/monitor_collector_main",
			"--cfg",
			"/opt/3fs/etc/monitor_collector_main.toml",
		},
	}
	_, err = s.Em.Docker.Run(ctx, args)
	if err != nil {
		return errors.Trace(err)
	}

	s.Logger.Infof("Start monitor container success")
	return nil
}

type rmContainerStep struct {
	task.BaseStep
}

func (s *rmContainerStep) Execute(ctx context.Context) error {
	containerName := s.Runtime.Services.Monitor.ContainerName
	s.Logger.Infof("Removing monitor container %s", containerName)
	_, err := s.Em.Docker.Rm(ctx, containerName, true)
	if err != nil {
		return errors.Trace(err)
	}
	etcDir := path.Join(s.Runtime.Services.Monitor.WorkDir, "etc")
	s.Logger.Infof("Remove monitor container etc dir %s", etcDir)
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", etcDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", etcDir)
	}
	logDir := path.Join(s.Runtime.Services.Monitor.WorkDir, "log")
	s.Logger.Infof("Remove monitor container log dir %s", logDir)
	_, err = s.Em.Runner.Exec(ctx, "rm", "-rf", logDir)
	if err != nil {
		return errors.Annotatef(err, "rm %s", logDir)
	}
	s.Logger.Infof("Monitor container %s successfully removed", containerName)
	return nil
}
