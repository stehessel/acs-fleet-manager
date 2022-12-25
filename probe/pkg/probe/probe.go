// Package probe ...
package probe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/stackrox/acs-fleet-manager/internal/dinosaur/constants"
	"github.com/stackrox/acs-fleet-manager/pkg/api/public"
	"github.com/stackrox/acs-fleet-manager/internal/dinosaur/pkg/dinosaurs/types"
	"github.com/stackrox/acs-fleet-manager/pkg/client/fleetmanager"
	"github.com/stackrox/acs-fleet-manager/probe/config"
	"github.com/stackrox/acs-fleet-manager/probe/pkg/metrics"
	"github.com/stackrox/rox/pkg/httputil"
)

// Probe is a wrapper interface for the core probe logic.
//
//go:generate moq -out probe_moq.go . Probe
type Probe interface {
	Execute(ctx context.Context) error
	CleanUp(ctx context.Context) error
}

var _ Probe = (*ProbeImpl)(nil)

// ProbeImpl executes a probe run against fleet manager.
type ProbeImpl struct {
	config                *config.Config
	fleetManagerPublicAPI fleetmanager.PublicAPI
	httpClient            *http.Client
}

// New creates a new probe.
func New(config *config.Config, fleetManagerPublicAPI fleetmanager.PublicAPI, httpClient *http.Client) *ProbeImpl {
	return &ProbeImpl{
		config:                config,
		fleetManagerPublicAPI: fleetManagerPublicAPI,
		httpClient:            httpClient,
	}
}

func recordElapsedTime(start time.Time) {
	elapsedTime := time.Since(start)
	glog.Infof("elapsed time: %v", elapsedTime)
	metrics.MetricsInstance().ObserveTotalDuration(elapsedTime)
}

func (p *ProbeImpl) newCentralName() (string, error) {
	rnd := make([]byte, 2)
	if _, err := rand.Read(rnd); err != nil {
		return "", errors.Wrapf(err, "reading random bytes for unique central name")
	}
	rndString := hex.EncodeToString(rnd)
	return fmt.Sprintf("%s-%s", p.config.ProbeName, rndString), nil
}

// Execute the probe of the fleet manager API.
func (p *ProbeImpl) Execute(ctx context.Context) error {
	glog.Info("probe run has been started")
	defer glog.Info("probe run has ended")
	defer recordElapsedTime(time.Now())

	central, err := p.createCentral(ctx)
	if err != nil {
		return err
	}

	if err := p.verifyCentral(ctx, central); err != nil {
		return err
	}

	return p.deleteCentral(ctx, central)
}

// CleanUp remaining probe resources.
func (p *ProbeImpl) CleanUp(ctx context.Context) error {
	if err := retryUntilSucceeded(ctx, p.cleanupFunc, p.config.ProbePollPeriod); err != nil {
		return errors.Wrap(err, "cleanup centrals failed")
	}
	return nil
}

func (p *ProbeImpl) cleanupFunc(ctx context.Context) error {
	centralList, _, err := p.fleetManagerPublicAPI.GetCentrals(ctx, nil)
	if err != nil {
		err = errors.Wrap(err, "could not list centrals")
		glog.Error(err)
		return err
	}

	success := true
	for _, central := range centralList.Items {
		central := central
		if central.Owner != p.config.ProbeUsername || !strings.HasPrefix(central.Name, p.config.ProbeName) {
			continue
		}
		if err := p.deleteCentral(ctx, &central); err != nil {
			glog.Warningf("failed to clean up central instance %s: %s", central.Id, err)
			success = false
		}
	}

	if success {
		glog.Info("finished clean up attempt of probe resources")
		return nil
	}
	return errors.New("central clean up not successful")
}

// Create a Central and verify that it transitioned to 'ready' state.
func (p *ProbeImpl) createCentral(ctx context.Context) (*public.CentralRequest, error) {
	centralName, err := p.newCentralName()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create central name")
	}
	request := public.CentralRequestPayload{
		Name:          centralName,
		MultiAz:       true,
		CloudProvider: p.config.DataCloudProvider,
		Region:        p.config.DataPlaneRegion,
	}
	central, _, err := p.fleetManagerPublicAPI.CreateCentral(ctx, true, request)
	glog.Infof("creation of central instance requested")
	if err != nil {
		return nil, errors.Wrap(err, "creation of central instance failed")
	}

	centralResp, err := p.ensureCentralState(ctx, &central, constants.CentralRequestStatusReady.String())
	if err != nil {
		return nil, errors.Wrapf(err, "central instance %s did not reach ready state", central.Id)
	}
	return centralResp, nil
}

// Verify that the Central instance has the expected properties and that the
// Central UI is reachable.
func (p *ProbeImpl) verifyCentral(ctx context.Context, centralRequest *public.CentralRequest) error {
	if centralRequest.InstanceType != types.STANDARD.String() {
		return errors.Errorf("central has wrong instance type: expected %s, got %s", types.STANDARD, centralRequest.InstanceType)
	}

	if err := p.pingURL(ctx, centralRequest.CentralUIURL); err != nil {
		return errors.Wrapf(err, "could not reach central UI URL of instance %s", centralRequest.Id)
	}
	return nil
}

// Delete the Central instance and verify that it transitioned to 'deprovision' state.
func (p *ProbeImpl) deleteCentral(ctx context.Context, centralRequest *public.CentralRequest) error {
	_, err := p.fleetManagerPublicAPI.DeleteCentralById(ctx, centralRequest.Id, true)
	glog.Infof("deletion of central instance %s requested", centralRequest.Id)
	if err != nil {
		return errors.Wrapf(err, "deletion of central instance %s failed", centralRequest.Id)
	}

	err = p.ensureCentralDeleted(ctx, centralRequest)
	if err != nil {
		return errors.Wrapf(err, "central instance %s with status %s could not be deleted", centralRequest.Id, centralRequest.Status)
	}
	return nil
}

func (p *ProbeImpl) ensureCentralState(ctx context.Context, centralRequest *public.CentralRequest, targetState string) (*public.CentralRequest, error) {
	funcWrapper := func(funcCtx context.Context) (*public.CentralRequest, error) {
		return p.ensureStateFunc(funcCtx, centralRequest, targetState)
	}
	centralResp, err := retryUntilSucceededWithResponse(ctx, funcWrapper, p.config.ProbePollPeriod)
	if err != nil {
		return nil, errors.Wrap(err, "ensure central state failed")
	}
	return centralResp, nil
}

func (p *ProbeImpl) ensureStateFunc(ctx context.Context, centralRequest *public.CentralRequest, targetState string) (*public.CentralRequest, error) {
	centralResp, _, err := p.fleetManagerPublicAPI.GetCentralById(ctx, centralRequest.Id)
	if err != nil {
		err = errors.Wrapf(err, "central instance %s not reachable", centralRequest.Id)
		glog.Error(err)
		return nil, err
	}

	if centralResp.Status == targetState {
		glog.Infof("central instance %s is in %q state", centralResp.Id, targetState)
		return &centralResp, nil
	}
	err = errors.Errorf("central instance %s not in target state %q", centralRequest.Id, targetState)
	glog.Warning(err)
	return nil, err
}

func (p *ProbeImpl) ensureCentralDeleted(ctx context.Context, centralRequest *public.CentralRequest) error {
	funcWrapper := func(funcCtx context.Context) error {
		return p.ensureDeletedFunc(funcCtx, centralRequest)
	}

	if err := retryUntilSucceeded(ctx, funcWrapper, p.config.ProbePollPeriod); err != nil {
		return errors.Wrap(err, "ensure central deleted failed")
	}
	return nil
}

func (p *ProbeImpl) ensureDeletedFunc(ctx context.Context, centralRequest *public.CentralRequest) error {
	_, response, err := p.fleetManagerPublicAPI.GetCentralById(ctx, centralRequest.Id)
	if err != nil {
		if response != nil && response.StatusCode == http.StatusNotFound {
			glog.Infof("central instance %s has been deleted", centralRequest.Id)
			return nil
		}
		err = errors.Wrapf(err, "central instance %s not reachable", centralRequest.Id)
		glog.Error(err)
		return err
	}
	err = errors.Errorf("central instance %s not deleted", centralRequest.Id)
	glog.Warning(err)
	return err
}

func (p *ProbeImpl) pingURL(ctx context.Context, url string) error {
	funcWrapper := func(funcCtx context.Context) error {
		return p.pingFunc(funcCtx, url)
	}
	if err := retryUntilSucceeded(ctx, funcWrapper, p.config.ProbePollPeriod); err != nil {
		return errors.Wrap(err, "URL ping failed")
	}
	return nil
}

func (p *ProbeImpl) pingFunc(ctx context.Context, url string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		err = errors.Wrap(err, "failed to create request")
		glog.Error(err)
		return err
	}
	response, err := p.httpClient.Do(request)
	if err != nil {
		err = errors.Wrap(err, "URL not reachable")
		glog.Error(err)
		return err
	}
	defer response.Body.Close()
	if !httputil.Is2xxStatusCode(response.StatusCode) {
		err = errors.Errorf("URL ping did not succeed: %s", response.Status)
		glog.Warning(err)
		return err
	}
	return nil
}

func retryUntilSucceeded(ctx context.Context, fn func(context.Context) error, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "retry failed")
		case <-ticker.C:
			if err := fn(ctx); err == nil {
				return nil
			}
		}
	}
}

func retryUntilSucceededWithResponse(ctx context.Context, fn func(context.Context) (*public.CentralRequest, error), interval time.Duration) (*public.CentralRequest, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx.Err(), "retry failed")
		case <-ticker.C:
			if centralResp, err := fn(ctx); err == nil {
				return centralResp, nil
			}
		}
	}
}
