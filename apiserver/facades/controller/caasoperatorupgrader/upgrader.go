// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasoperatorupgrader

import (
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/juju/names.v2"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/caas"
	"github.com/juju/juju/environs/bootstrap"
	"github.com/juju/juju/state/stateenvirons"
)

var logger = loggo.GetLogger("juju.controller.caasoperatorupgrader")

type API struct {
	auth facade.Authorizer

	broker caas.Upgrader
}

// NewStateCAASOperatorUpgraderAPI provides the signature required for facade registration.
func NewStateCAASOperatorUpgraderAPI(ctx facade.Context) (*API, error) {
	authorizer := ctx.Auth()
	broker, err := stateenvirons.GetNewCAASBrokerFunc(caas.New)(ctx.State())
	if err != nil {
		return nil, errors.Annotate(err, "getting caas client")
	}
	return NewCAASOperatorUpgraderAPI(authorizer, broker)
}

// NewCAASOperatorUpgraderAPI returns a new CAAS operator upgrader API facade.
func NewCAASOperatorUpgraderAPI(
	authorizer facade.Authorizer,
	broker caas.Upgrader,
) (*API, error) {
	if !authorizer.AuthController() && !authorizer.AuthApplicationAgent() {
		return nil, common.ErrPerm
	}
	return &API{
		auth:   authorizer,
		broker: broker,
	}, nil
}

// OperatorVersion returns operator version for specified application.
func (api *API) OperatorVersion(args params.Entities) (params.VersionResults, error) {
	result := make([]params.VersionResult, len(args.Entities))
	for i, entity := range args.Entities {
		tag, err := names.ParseTag(entity.Tag)
		if err != nil {
			result[i].Error = common.ServerError(common.ErrPerm)
			continue
		}
		err = common.ErrPerm
		if api.auth.AuthOwner(tag) {
			*result[i].Version, err = api.broker.OperatorVersion(tagToAppName(tag))
		}
		result[i].Error = common.ServerError(err)
	}
	return params.VersionResults{Results: result}, nil
}

func tagToAppName(t names.Tag) string {
	appName := t.Id()
	// Machines representing controllers really mean the controller operator.
	if t.Kind() == names.MachineTagKind {
		appName = bootstrap.ControllerModelName
	}
	return appName
}

// UpgradeOperator upgrades the operator for the specified agents.
func (api *API) UpgradeOperator(arg params.KubernetesUpgradeArg) (params.ErrorResult, error) {
	serverErr := func(err error) params.ErrorResult {
		return params.ErrorResult{common.ServerError(err)}
	}
	tag, err := names.ParseTag(arg.AgentTag)
	if err != nil {
		return serverErr(err), nil
	}
	if !api.auth.AuthOwner(tag) {
		return serverErr(common.ErrPerm), nil
	}
	appName := tagToAppName(tag)
	logger.Debugf("upgrading caas app %v", appName)
	err = api.broker.Upgrade(appName, arg.Version)
	if err != nil {
		return serverErr(err), nil
	}
	return params.ErrorResult{}, nil
}
