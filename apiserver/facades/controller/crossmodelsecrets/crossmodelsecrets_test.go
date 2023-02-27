// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package crossmodelsecrets_test

import (
	"context"
	"time"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery"
	"github.com/go-macaroon-bakery/macaroon-bakery/v3/bakery/checkers"
	"github.com/golang/mock/gomock"
	"github.com/juju/errors"
	"github.com/juju/names/v4"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/macaroon.v2"

	"github.com/juju/juju/apiserver/authentication"
	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/common/crossmodel"
	"github.com/juju/juju/apiserver/facades/controller/crossmodelsecrets"
	"github.com/juju/juju/apiserver/facades/controller/crossmodelsecrets/mocks"
	coresecrets "github.com/juju/juju/core/secrets"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/secrets/provider"
	coretesting "github.com/juju/juju/testing"
)

var _ = gc.Suite(&CrossModelSecretsSuite{})

type CrossModelSecretsSuite struct {
	coretesting.BaseSuite

	resources       *common.Resources
	secretsState    *mocks.MockSecretsState
	secretsConsumer *mocks.MockSecretsConsumer
	crossModelState *mocks.MockCrossModelState
	stateBackend    *mocks.MockStateBackend

	facade *crossmodelsecrets.CrossModelSecretsAPI

	authContext *crossmodel.AuthContext
	bakery      authentication.ExpirableStorageBakery
}

type testLocator struct {
	PublicKey bakery.PublicKey
}

func (b testLocator) ThirdPartyInfo(ctx context.Context, loc string) (bakery.ThirdPartyInfo, error) {
	if loc != "http://thirdparty" {
		return bakery.ThirdPartyInfo{}, errors.NotFoundf("location %v", loc)
	}
	return bakery.ThirdPartyInfo{
		PublicKey: b.PublicKey,
		Version:   bakery.LatestVersion,
	}, nil
}

type mockBakery struct {
	*bakery.Bakery
}

func (m *mockBakery) ExpireStorageAfter(_ time.Duration) (authentication.ExpirableStorageBakery, error) {
	return m, nil
}

func (m *mockBakery) Auth(mss ...macaroon.Slice) *bakery.AuthChecker {
	return m.Bakery.Checker.Auth(mss...)
}

func (m *mockBakery) NewMacaroon(ctx context.Context, version bakery.Version, caveats []checkers.Caveat, ops ...bakery.Op) (*bakery.Macaroon, error) {
	return m.Bakery.Oven.NewMacaroon(ctx, version, caveats, ops...)
}

func (s *CrossModelSecretsSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.resources = common.NewResources()
	s.AddCleanup(func(*gc.C) { s.resources.StopAll() })

	key, err := bakery.GenerateKey()
	c.Assert(err, jc.ErrorIsNil)
	locator := testLocator{key.Public}
	bakery := bakery.New(bakery.BakeryParams{
		Locator:       locator,
		Key:           bakery.MustGenerateKey(),
		OpsAuthorizer: crossmodel.CrossModelAuthorizer{},
	})
	s.bakery = &mockBakery{bakery}
	s.authContext, err = crossmodel.NewAuthContext(nil, key, s.bakery)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *CrossModelSecretsSuite) setup(c *gc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)

	s.secretsState = mocks.NewMockSecretsState(ctrl)
	s.secretsConsumer = mocks.NewMockSecretsConsumer(ctrl)
	s.crossModelState = mocks.NewMockCrossModelState(ctrl)
	s.stateBackend = mocks.NewMockStateBackend(ctrl)

	secretsStateGetter := func(modelUUID string) (crossmodelsecrets.SecretsState, crossmodelsecrets.SecretsConsumer, func() bool, error) {
		return s.secretsState, s.secretsConsumer, func() bool { return false }, nil
	}
	backendConfigGetter := func(modelUUID string) (*provider.ModelBackendConfigInfo, error) {
		return &provider.ModelBackendConfigInfo{
			ActiveID: "active-id",
			Configs: map[string]provider.ModelBackendConfig{
				"backend-id": {
					ControllerUUID: coretesting.ControllerTag.Id(),
					ModelUUID:      modelUUID,
					ModelName:      "fred",
					BackendConfig: provider.BackendConfig{
						BackendType: "vault",
						Config:      map[string]interface{}{"foo": "bar"},
					},
				},
			},
		}, nil
	}
	var err error
	s.facade, err = crossmodelsecrets.NewCrossModelSecretsAPI(
		s.resources,
		s.authContext,
		coretesting.ModelTag.Id(),
		secretsStateGetter,
		backendConfigGetter,
		s.crossModelState,
		s.stateBackend,
	)
	c.Assert(err, jc.ErrorIsNil)

	return ctrl
}

func ptr[T any](v T) *T {
	return &v
}

func (s *CrossModelSecretsSuite) TestGetSecretContentInfo(c *gc.C) {
	defer s.setup(c).Finish()

	uri := coresecrets.NewURI().WithSource(coretesting.ModelTag.Id())
	app := names.NewApplicationTag("remote-app")
	consumer := names.NewUnitTag("remote-app/666")
	relation := names.NewRelationTag("remote-app:foo local-app:foo")
	offerUUID := "offer-uuid"
	s.crossModelState.EXPECT().GetConsumerInfo("token").Return(app, offerUUID, nil)
	s.stateBackend.EXPECT().HasEndpoint(relation.Id(), "remote-app").Return(true, nil)

	// Remote app 2 has mismatched offer.
	app2 := names.NewApplicationTag("remote-app2")
	offerUUID2 := "offer-uuid2"
	s.crossModelState.EXPECT().GetConsumerInfo("token2").Return(app2, offerUUID2, nil)

	// Remote app 3 has incorrect relation.
	app3 := names.NewApplicationTag("remote-app3")
	s.crossModelState.EXPECT().GetConsumerInfo("token3").Return(app3, offerUUID, nil)
	s.stateBackend.EXPECT().HasEndpoint(relation.Id(), "remote-app3").Return(false, nil)

	s.secretsState.EXPECT().GetSecret(uri).Return(&coresecrets.SecretMetadata{
		LatestRevision: 667,
	}, nil)
	s.secretsConsumer.EXPECT().SecretAccess(uri, consumer).Return(coresecrets.RoleView, nil)
	s.secretsState.EXPECT().GetSecretValue(uri, 667).Return(
		nil,
		&coresecrets.ValueRef{
			BackendID:  "backend-id",
			RevisionID: "rev-id",
		}, nil,
	)

	mac, err := s.bakery.NewMacaroon(
		context.TODO(),
		bakery.LatestVersion,
		[]checkers.Caveat{
			checkers.DeclaredCaveat("username", "mary"),
			checkers.DeclaredCaveat("offer-uuid", offerUUID),
			checkers.DeclaredCaveat("source-model-uuid", coretesting.ModelTag.Id()),
			checkers.DeclaredCaveat("relation-key", relation.Id()),
		}, bakery.Op{"consume", "mysql-uuid"})
	c.Assert(err, jc.ErrorIsNil)

	args := params.GetRemoteSecretContentArgs{
		Args: []params.GetRemoteSecretContentArg{{
			ApplicationToken: "token",
			UnitId:           666,
			BakeryVersion:    3,
			Macaroons:        macaroon.Slice{mac.M()},
			URI:              uri.String(),
			Refresh:          true,
		}, {
			URI: coresecrets.NewURI().String(),
		}, {
			URI: uri.String(),
		}, {
			ApplicationToken: "token2",
			UnitId:           666,
			BakeryVersion:    3,
			Macaroons:        macaroon.Slice{mac.M()},
			URI:              uri.String(),
			Refresh:          true,
		}, {
			ApplicationToken: "token3",
			UnitId:           666,
			BakeryVersion:    3,
			Macaroons:        macaroon.Slice{mac.M()},
			URI:              uri.String(),
			Refresh:          true,
		}},
	}
	results, err := s.facade.GetSecretContentInfo(args)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(results, jc.DeepEquals, params.SecretContentResults{
		Results: []params.SecretContentResult{{
			Content: params.SecretContentParams{
				ValueRef: &params.SecretValueRef{
					BackendID:  "backend-id",
					RevisionID: "rev-id",
				},
			},
			BackendConfig: &params.SecretBackendConfigResult{
				ControllerUUID: coretesting.ControllerTag.Id(),
				ModelUUID:      coretesting.ModelTag.Id(),
				ModelName:      "fred",
				Draining:       true,
				Config: params.SecretBackendConfig{
					BackendType: "vault",
					Params:      map[string]interface{}{"foo": "bar"},
				},
			},
			LatestRevision: ptr(667),
		}, {
			Error: &params.Error{
				Code:    "not valid",
				Message: "secret URI with empty source UUID not valid",
			},
		}, {
			Error: &params.Error{
				Code:    "not valid",
				Message: "empty secret revision not valid",
			},
		}, {
			Error: &params.Error{
				Code:    "unauthorized access",
				Message: "permission denied",
			},
		}, {
			Error: &params.Error{
				Code:    "unauthorized access",
				Message: "permission denied",
			},
		}},
	})
}