// pmm-managed
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package prometheus contains business logic of working with Prometheus.
package prometheus

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/reform.v1"
	"gopkg.in/yaml.v2"

	"github.com/percona/pmm-managed/models"
	"github.com/percona/pmm-managed/services/prometheus/internal/prometheus/config"
	"github.com/percona/pmm-managed/utils/logger"
)

var checkFailedRE = regexp.MustCompile(`FAILED: parsing YAML file \S+: (.+)\n`)

// Service is responsible for interactions with Prometheus.
// It assumes the following:
//   * Prometheus APIs (including lifecycle) are accessible;
//   * Prometheus configuration and rule files are accessible;
//   * promtool is available.
type Service struct {
	configPath   string
	promtoolPath string
	db           *reform.DB
	baseURL      *url.URL
	client       *http.Client

	configM sync.Mutex
}

// NewService creates new service.
func NewService(configPath string, promtoolPath string, db *reform.DB, baseURL string) (*Service, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &Service{
		configPath:   configPath,
		promtoolPath: promtoolPath,
		db:           db,
		baseURL:      u,
		client:       new(http.Client),
	}, nil
}

// reload asks Prometheus to reload configuration.
func (svc *Service) reload() error {
	u := *svc.baseURL
	u.Path = path.Join(u.Path, "-", "reload")
	resp, err := svc.client.Post(u.String(), "", nil)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode == 200 {
		return nil
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.Errorf("%d: %s", resp.StatusCode, b)
}

// marshalConfig marshals Prometheus configuration.
func (svc *Service) marshalConfig(ctx context.Context) ([]byte, error) {
	l := logger.Get(ctx).WithField("component", "prometheus")

	cfg := &config.Config{
		GlobalConfig: config.GlobalConfig{
			ScrapeInterval:     model.Duration(time.Minute),
			ScrapeTimeout:      model.Duration(10 * time.Second),
			EvaluationInterval: model.Duration(time.Minute),
		},
		RuleFiles: []string{
			"/etc/prometheus.d/*.rules.yml",
		},
		ScrapeConfigs: []*config.ScrapeConfig{
			scrapeConfigForPrometheus(),
			scrapeConfigForGrafana(),
			scrapeConfigForPMMManaged(),
		},
	}

	e := svc.db.InTransaction(func(tx *reform.TX) error {
		agents, err := tx.SelectAllFrom(models.AgentTable, "ORDER BY agent_id")
		if err != nil {
			return errors.WithStack(err)
		}
		for _, str := range agents {
			agent := str.(*models.Agent)
			services, err := models.ServicesForAgent(tx.Querier, agent.AgentID)
			if err != nil {
				return errors.WithStack(err)
			}
			for _, service := range services {

				node := &models.Node{NodeID: service.NodeID}
				if err = tx.Reload(node); err != nil {
					return errors.WithStack(err)
				}

				switch service.ServiceType {
				case models.MySQLServiceType:
					scfgs, err := scrapeConfigsForMySQLdExporter(node, service, agent)
					if err != nil {
						return err
					}
					cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, scfgs...)
				case models.MongoDBServiceType:
					scfg, err := scrapeConfigsForMongoDBExporter(node, service, agent)
					if err != nil {
						return err
					}
					cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, scfg)
				default:
					l.Warnf("Skipping scrape config for %s.", service)
					continue
				}
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}

	// TODO Add comments to each cfg.ScrapeConfigs element.
	// https://jira.percona.com/browse/PMM-3601

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal Prometheus configuration file")
	}

	b = append([]byte("# Managed by pmm-managed. DO NOT EDIT.\n---\n"), b...)
	return b, nil
}

// saveConfigAndReload saves given Prometheus configuration to file and reloads Prometheus.
// If configuration can't be reloaded for some reason, old file is restored, and configuration is reloaded again.
func (svc *Service) saveConfigAndReload(ctx context.Context, cfg []byte) error {
	l := logger.Get(ctx).WithField("component", "prometheus")

	// read existing content
	oldCfg, err := ioutil.ReadFile(svc.configPath)
	if err != nil {
		return errors.WithStack(err)
	}

	// compare with new config
	if reflect.DeepEqual(cfg, oldCfg) {
		l.Infof("Configuration not changed, doing nothing.")
		return nil
	}

	fi, err := os.Stat(svc.configPath)
	if err != nil {
		return errors.WithStack(err)
	}

	// restore old content and reload in case of error
	var restore bool
	defer func() {
		if restore {
			if err = ioutil.WriteFile(svc.configPath, oldCfg, fi.Mode()); err != nil {
				l.Error(err)
			}
			if err = svc.reload(); err != nil {
				l.Error(err)
			}
		}
	}()

	// write new content to temporary file, check it
	f, err := ioutil.TempFile("", "pmm-managed-config-")
	if err != nil {
		return errors.WithStack(err)
	}
	if _, err = f.Write(cfg); err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	args := []string{"check", "config", f.Name()}
	b, err := exec.CommandContext(ctx, svc.promtoolPath, args...).CombinedOutput() //nolint:gosec
	if err != nil {
		l.Errorf("%s", b)

		// return typed error if possible
		s := string(b)
		if m := checkFailedRE.FindStringSubmatch(s); len(m) == 2 {
			return status.Error(codes.Aborted, m[1])
		}
		return errors.Wrap(err, s)
	}
	l.Debugf("%s", b)

	// write to permanent location and reload
	restore = true
	if err = ioutil.WriteFile(svc.configPath, cfg, fi.Mode()); err != nil {
		return errors.WithStack(err)
	}
	if err = svc.reload(); err != nil {
		return err
	}
	l.Infof("Configuration reloaded.")
	restore = false
	return nil
}

// UpdateConfiguration updates Prometheus configuration.
func (svc *Service) UpdateConfiguration(ctx context.Context) error {
	svc.configM.Lock()
	defer svc.configM.Unlock()

	cfg, err := svc.marshalConfig(ctx)
	if err != nil {
		return err
	}
	return svc.saveConfigAndReload(ctx, cfg)
}

// Check verifies that Prometheus works.
func (svc *Service) Check(ctx context.Context) error {
	l := logger.Get(ctx).WithField("component", "prometheus")

	// check Prometheus /version API and log version
	u := *svc.baseURL
	u.Path = path.Join(u.Path, "version")
	resp, err := svc.client.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	l.Debugf("Prometheus: %s", b)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// check promtool version
	b, err = exec.CommandContext(ctx, svc.promtoolPath, "--version").CombinedOutput() //nolint:gosec
	if err != nil {
		return errors.Wrap(err, string(b))
	}
	l.Debugf("%s", b)

	return svc.UpdateConfiguration(ctx)
}