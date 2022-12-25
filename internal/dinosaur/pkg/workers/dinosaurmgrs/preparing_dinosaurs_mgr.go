package dinosaurmgrs

import (
	"time"

	"github.com/stackrox/acs-fleet-manager/internal/dinosaur/pkg/config"

	"github.com/google/uuid"
	constants2 "github.com/stackrox/acs-fleet-manager/internal/dinosaur/constants"
	"github.com/stackrox/acs-fleet-manager/pkg/api/dbapi"
	"github.com/stackrox/acs-fleet-manager/internal/dinosaur/pkg/services"

	"github.com/pkg/errors"
	"github.com/stackrox/acs-fleet-manager/pkg/metrics"
	"github.com/stackrox/acs-fleet-manager/pkg/workers"

	"github.com/golang/glog"

	serviceErr "github.com/stackrox/acs-fleet-manager/pkg/errors"
)

// PreparingDinosaurManager represents a dinosaur manager that periodically reconciles dinosaur requests
type PreparingDinosaurManager struct {
	workers.BaseWorker
	dinosaurService       services.DinosaurService
	centralRequestTimeout time.Duration
}

// NewPreparingDinosaurManager creates a new dinosaur manager
func NewPreparingDinosaurManager(dinosaurService services.DinosaurService, centralConfig *config.CentralConfig) *PreparingDinosaurManager {
	return &PreparingDinosaurManager{
		BaseWorker: workers.BaseWorker{
			ID:         uuid.New().String(),
			WorkerType: "preparing_dinosaur",
			Reconciler: workers.Reconciler{},
		},
		dinosaurService:       dinosaurService,
		centralRequestTimeout: centralConfig.CentralRequestExpirationTimeout,
	}
}

// Start initializes the dinosaur manager to reconcile dinosaur requests
func (k *PreparingDinosaurManager) Start() {
	k.StartWorker(k)
}

// Stop causes the process for reconciling dinosaur requests to stop.
func (k *PreparingDinosaurManager) Stop() {
	k.StopWorker(k)
}

// Reconcile ...
func (k *PreparingDinosaurManager) Reconcile() []error {
	glog.Infoln("reconciling preparing centrals")
	var encounteredErrors []error

	// handle preparing dinosaurs
	preparingDinosaurs, serviceErr := k.dinosaurService.ListByStatus(constants2.CentralRequestStatusPreparing)
	if serviceErr != nil {
		encounteredErrors = append(encounteredErrors, errors.Wrap(serviceErr, "failed to list preparing centrals"))
	} else {
		glog.Infof("preparing centrals count = %d", len(preparingDinosaurs))
	}

	for _, dinosaur := range preparingDinosaurs {
		glog.V(10).Infof("preparing central id = %s", dinosaur.ID)
		metrics.UpdateCentralRequestsStatusSinceCreatedMetric(constants2.CentralRequestStatusPreparing, dinosaur.ID, dinosaur.ClusterID, time.Since(dinosaur.CreatedAt))
		if err := k.reconcilePreparingDinosaur(dinosaur); err != nil {
			encounteredErrors = append(encounteredErrors, errors.Wrapf(err, "failed to reconcile preparing central %s", dinosaur.ID))
			continue
		}

	}

	return encounteredErrors
}

func (k *PreparingDinosaurManager) reconcilePreparingDinosaur(dinosaur *dbapi.CentralRequest) error {
	// Check if instance creation is not expired before trying to reconcile it.
	// Otherwise, assign status Failed.
	if err := FailIfTimeoutExceeded(k.dinosaurService, k.centralRequestTimeout, dinosaur); err != nil {
		return err
	}
	if err := k.dinosaurService.PrepareDinosaurRequest(dinosaur); err != nil {
		return k.handleDinosaurRequestCreationError(dinosaur, err)
	}

	return nil
}

func (k *PreparingDinosaurManager) handleDinosaurRequestCreationError(dinosaurRequest *dbapi.CentralRequest, err *serviceErr.ServiceError) error {
	if err.IsServerErrorClass() {
		// retry the dinosaur creation request only if the failure is caused by server errors
		// and the time elapsed since its db record was created is still within the threshold.
		durationSinceCreation := time.Since(dinosaurRequest.CreatedAt)
		if durationSinceCreation > constants2.CentralMaxDurationWithProvisioningErrs {
			metrics.IncreaseCentralTotalOperationsCountMetric(constants2.CentralOperationCreate)
			dinosaurRequest.Status = string(constants2.CentralRequestStatusFailed)
			dinosaurRequest.FailedReason = err.Reason
			updateErr := k.dinosaurService.Update(dinosaurRequest)
			if updateErr != nil {
				return errors.Wrapf(updateErr, "Failed to update central %s in failed state. Central failed reason %s", dinosaurRequest.ID, dinosaurRequest.FailedReason)
			}
			metrics.UpdateCentralRequestsStatusSinceCreatedMetric(constants2.CentralRequestStatusFailed, dinosaurRequest.ID, dinosaurRequest.ClusterID, time.Since(dinosaurRequest.CreatedAt))
			return errors.Wrapf(err, "Central %s is in server error failed state. Maximum attempts has been reached", dinosaurRequest.ID)
		}
	} else if err.IsClientErrorClass() {
		metrics.IncreaseCentralTotalOperationsCountMetric(constants2.CentralOperationCreate)
		dinosaurRequest.Status = string(constants2.CentralRequestStatusFailed)
		dinosaurRequest.FailedReason = err.Reason
		updateErr := k.dinosaurService.Update(dinosaurRequest)
		if updateErr != nil {
			return errors.Wrapf(err, "Failed to update central %s in failed state", dinosaurRequest.ID)
		}
		metrics.UpdateCentralRequestsStatusSinceCreatedMetric(constants2.CentralRequestStatusFailed, dinosaurRequest.ID, dinosaurRequest.ClusterID, time.Since(dinosaurRequest.CreatedAt))
		return errors.Wrapf(err, "error creating central %s", dinosaurRequest.ID)
	}

	return errors.Wrapf(err, "failed to provision central %s on cluster %s", dinosaurRequest.ID, dinosaurRequest.ClusterID)
}
