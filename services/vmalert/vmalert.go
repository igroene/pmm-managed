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

// Package vmalert provides facilities for working with VMAlert.
package vmalert

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/pkg/errors"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/percona/pmm-managed/utils/irt"
)

const (
	updateBatchDelay           = 3 * time.Second
	configurationUpdateTimeout = 2 * time.Second
)

// Service is responsible for interactions with VMAlert.
type Service struct {
	alertingRules *ExternalAlertingRules

	baseURL *url.URL
	client  *http.Client
	irtm    prom.Collector
	l       *logrus.Entry
	sema    chan struct{}
}

// Type represents VMAlert instance type.
type Type string

const (
	// Integrated is a VMAlert for Integrated Alerting.
	Integrated = Type("integrated")

	// External is a VMAlert for external Alertmanager.
	External = Type("external")
)

// NewVMAlert creates new VMAlert service.
func NewVMAlert(alertRules *ExternalAlertingRules, typ Type) (*Service, error) {
	var baseURL string
	switch typ {
	case Integrated:
		baseURL = "http://127.0.0.1:8880/"
	case External:
		baseURL = "http://127.0.0.1:8881/"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse URL for %s VMAlert", typ)
	}

	subsystem := "vmalert_" + string(typ)
	var t http.RoundTripper = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          1,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if logrus.GetLevel() >= logrus.TraceLevel {
		t = irt.WithLogger(t, logrus.WithField("component", subsystem+"/client").Tracef)
	}
	t, irtm := irt.WithMetrics(t, subsystem)

	return &Service{
		alertingRules: alertRules,
		baseURL:       u,
		client: &http.Client{
			Transport: t,
		},
		irtm: irtm,
		l:    logrus.WithField("component", subsystem),
		sema: make(chan struct{}, 1),
	}, nil
}

// Describe implements prometheus.Collector.
func (svc *Service) Describe(ch chan<- *prom.Desc) {
	svc.irtm.Describe(ch)
}

// Collect implements prometheus.Collector.
func (svc *Service) Collect(ch chan<- prom.Metric) {
	svc.irtm.Collect(ch)
}

// Run runs VMAlert configuration update loop until ctx is canceled.
func (svc *Service) Run(ctx context.Context) {
	svc.l.Info("Starting...")
	defer svc.l.Info("Done.")

	for {
		select {
		case <-ctx.Done():
			return

		case <-svc.sema:
			// batch several update requests together by delaying the first one
			sleepCtx, sleepCancel := context.WithTimeout(ctx, updateBatchDelay)
			<-sleepCtx.Done()
			sleepCancel()

			if ctx.Err() != nil {
				return
			}

			if err := svc.updateConfiguration(ctx); err != nil {
				svc.l.Errorf("Failed to update configuration, will retry: %+v.", err)
				svc.RequestConfigurationUpdate()
			}
		}
	}
}

// RequestConfigurationUpdate requests VMAlert configuration update.
func (svc *Service) RequestConfigurationUpdate() {
	select {
	case svc.sema <- struct{}{}:
		ctx, cancel := context.WithTimeout(context.Background(), configurationUpdateTimeout)
		defer cancel()
		err := svc.updateConfiguration(ctx)
		if err != nil {
			svc.l.WithError(err).Errorf("cannot reload configuration")
		}
	default:
	}
}

// IsReady verifies that VMAlert works.
func (svc *Service) IsReady(ctx context.Context) error {
	u := *svc.baseURL
	u.Path = path.Join(u.Path, "health")
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return errors.WithStack(err)
	}
	resp, err := svc.client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	b, err := ioutil.ReadAll(resp.Body)
	svc.l.Debugf("VMAlert health: %s", b)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.Errorf("expected 200, got %d", resp.StatusCode)
	}
	return nil
}

// reload asks VMAlert to reload configuration.
func (svc *Service) reload(ctx context.Context) error {
	u := *svc.baseURL
	u.Path = path.Join(u.Path, "-", "reload")
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return errors.WithStack(err)
	}
	resp, err := svc.client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	b, err := ioutil.ReadAll(resp.Body)
	svc.l.Debugf("VMAlert reload: %s", b)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.Errorf("expected 200, got %d", resp.StatusCode)
	}
	return nil
}

// updateConfiguration updates VMAlert configuration.
func (svc *Service) updateConfiguration(ctx context.Context) error {
	start := time.Now()
	defer func() {
		if dur := time.Since(start); dur > time.Second {
			svc.l.Warnf("updateConfiguration took %s.", dur)
		}
	}()

	// Currently, we generate rule files in other services, don't call RequestConfigurationUpdate too often,
	// and don't have problems Prometheus had with often configuration reloads, so we just reload it.
	// We might want to add checks to avoid reloading if rules did not change later.

	if err := svc.reload(ctx); err != nil {
		return errors.WithStack(err)
	}
	svc.l.Infof("Configuration reloaded.")

	return nil
}

// Check interfaces.
var (
	_ prom.Collector = (*Service)(nil)
)