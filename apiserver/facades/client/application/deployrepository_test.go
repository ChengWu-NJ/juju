// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package application

import (
	"github.com/golang/mock/gomock"
	"github.com/juju/charm/v10"
	"github.com/juju/charm/v10/resource"
	"github.com/juju/errors"
	jc "github.com/juju/testing/checkers"
	"github.com/kr/pretty"
	gc "gopkg.in/check.v1"

	corecharm "github.com/juju/juju/core/charm"
	coreconfig "github.com/juju/juju/core/config"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/juju/state"
	coretesting "github.com/juju/juju/testing"
)

type validatorSuite struct {
	bindings    *MockBindings
	machine     *MockMachine
	model       *MockModel
	repo        *MockRepository
	repoFactory *MockRepositoryFactory
	state       *MockDeployFromRepositoryState
}

var _ = gc.Suite(&deployRepositorySuite{})
var _ = gc.Suite(&validatorSuite{})

func (s *validatorSuite) TestValidateSuccess(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.expectSimpleValidate()
	// resolveCharm
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	// getCharm
	expMeta := &charm.Meta{
		Name: "test-charm",
	}
	expManifest := new(charm.Manifest)
	expConfig := new(charm.Config)
	essMeta := corecharm.EssentialMetadata{
		Meta:           expMeta,
		Manifest:       expManifest,
		Config:         expConfig,
		ResolvedOrigin: resolvedOrigin,
	}
	s.repo.EXPECT().GetEssentialMetadata(corecharm.MetadataRequest{
		CharmURL: resultURL,
		Origin:   resolvedOrigin,
	}).Return([]corecharm.EssentialMetadata{essMeta}, nil)
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("arm64")}, nil)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testcharm",
	}
	dt, errs := s.getValidator().validate(arg)
	c.Assert(errs, gc.HasLen, 0)
	c.Assert(dt, gc.DeepEquals, deployTemplate{
		applicationName: "test-charm",
		charm:           corecharm.NewCharmInfoAdapter(essMeta),
		charmURL:        resultURL,
		numUnits:        1,
		origin:          resolvedOrigin,
	})
}

func (s *validatorSuite) TestValidatePlacementSuccess(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.expectSimpleValidate()
	// resolveCharm
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	// getCharm
	expMeta := &charm.Meta{
		Name: "test-charm",
	}
	expManifest := new(charm.Manifest)
	expConfig := new(charm.Config)
	essMeta := corecharm.EssentialMetadata{
		Meta:           expMeta,
		Manifest:       expManifest,
		Config:         expConfig,
		ResolvedOrigin: resolvedOrigin,
	}
	s.repo.EXPECT().GetEssentialMetadata(corecharm.MetadataRequest{
		CharmURL: resultURL,
		Origin:   resolvedOrigin,
	}).Return([]corecharm.EssentialMetadata{essMeta}, nil)

	// Placement
	s.state.EXPECT().Machine("0").Return(s.machine, nil).Times(2)
	s.machine.EXPECT().IsLockedForSeriesUpgrade().Return(false, nil)
	s.machine.EXPECT().IsParentLockedForSeriesUpgrade().Return(false, nil)
	s.machine.EXPECT().Base().Return(state.Base{
		OS:      "ubuntu",
		Channel: "22.04",
	})
	hwc := &instance.HardwareCharacteristics{Arch: strptr("amd64")}
	s.machine.EXPECT().HardwareCharacteristics().Return(hwc, nil)
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("arm64")}, nil)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testcharm",
		Placement: []*instance.Placement{{Directive: "0", Scope: instance.MachineScope}},
	}
	dt, errs := s.getValidator().validate(arg)
	c.Assert(errs, gc.HasLen, 0)
	c.Assert(dt, gc.DeepEquals, deployTemplate{
		applicationName: "test-charm",
		charm:           corecharm.NewCharmInfoAdapter(essMeta),
		charmURL:        resultURL,
		numUnits:        1,
		origin:          resolvedOrigin,
		placement:       arg.Placement,
	})
}

func (s *validatorSuite) TestValidateEndpointBindingSuccess(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.expectSimpleValidate()
	// resolveCharm
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	// getCharm
	expMeta := &charm.Meta{
		Name: "test-charm",
	}
	expManifest := new(charm.Manifest)
	expConfig := new(charm.Config)
	essMeta := corecharm.EssentialMetadata{
		Meta:           expMeta,
		Manifest:       expManifest,
		Config:         expConfig,
		ResolvedOrigin: resolvedOrigin,
	}
	s.repo.EXPECT().GetEssentialMetadata(corecharm.MetadataRequest{
		CharmURL: resultURL,
		Origin:   resolvedOrigin,
	}).Return([]corecharm.EssentialMetadata{essMeta}, nil)

	// state bindings
	endpointMap := map[string]string{"to": "from"}
	s.bindings.EXPECT().Map().Return(endpointMap)
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("arm64")}, nil)

	arg := params.DeployFromRepositoryArg{
		CharmName:        "testcharm",
		EndpointBindings: endpointMap,
	}
	dt, errs := s.getValidator().validate(arg)
	c.Assert(errs, gc.HasLen, 0)
	c.Assert(dt, gc.DeepEquals, deployTemplate{
		applicationName: "test-charm",
		charm:           corecharm.NewCharmInfoAdapter(essMeta),
		charmURL:        resultURL,
		endpoints:       endpointMap,
		numUnits:        1,
		origin:          resolvedOrigin,
	})
}

func (s *validatorSuite) expectSimpleValidate() {
	// createOrigin
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig())).AnyTimes()
}

func (s *validatorSuite) TestResolveCharm(c *gc.C) {
	defer s.setupMocks(c).Finish()
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{
		Arch: strptr("arm64"),
	}, nil)

	obtainedCurl, obtainedOrigin, err := s.getValidator().resolveCharm(curl, origin, false, false, constraints.Value{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(obtainedCurl, gc.DeepEquals, resultURL)
	c.Assert(obtainedOrigin, gc.DeepEquals, resolvedOrigin)
}

func (s *validatorSuite) TestResolveCharmArchAll(c *gc.C) {
	defer s.setupMocks(c).Finish()
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "all", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("arm64")}, nil)

	obtainedCurl, obtainedOrigin, err := s.getValidator().resolveCharm(curl, origin, false, false, constraints.Value{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(obtainedCurl, gc.DeepEquals, resultURL)
	expectedOrigin := resolvedOrigin
	expectedOrigin.Platform.Architecture = "arm64"
	c.Assert(obtainedOrigin, gc.DeepEquals, expectedOrigin)
}

func (s *validatorSuite) TestResolveCharmUnsupportedSeriesErrorForce(c *gc.C) {
	defer s.setupMocks(c).Finish()
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"focal"}
	newErr := charm.NewUnsupportedSeriesError("jammy", supportedSeries)
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, newErr)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("arm64")}, nil)

	obtainedCurl, obtainedOrigin, err := s.getValidator().resolveCharm(curl, origin, true, false, constraints.Value{})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(obtainedCurl, gc.DeepEquals, resultURL)
	c.Assert(obtainedOrigin, gc.DeepEquals, resolvedOrigin)
}

func (s *validatorSuite) TestResolveCharmUnsupportedSeriesError(c *gc.C) {
	defer s.setupMocks(c).Finish()
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"focal"}
	newErr := charm.NewUnsupportedSeriesError("jammy", supportedSeries)
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, newErr)

	_, _, err := s.getValidator().resolveCharm(curl, origin, false, false, constraints.Value{})
	c.Assert(err, gc.ErrorMatches, `series "jammy" not supported by charm, supported series are: focal. Use --force to deploy the charm anyway.`)
}

func (s *validatorSuite) TestResolveCharmExplicitBaseErrorWhenUserImageID(c *gc.C) {
	defer s.setupMocks(c).Finish()
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("arm64")}, nil)

	_, _, err := s.getValidator().resolveCharm(curl, origin, false, false, constraints.Value{ImageID: strptr("ubuntu-bf2")})
	c.Assert(err, gc.ErrorMatches, `base must be explicitly provided when image-id constraint is used`)
}

func (s *validatorSuite) TestResolveCharmExplicitBaseErrorWhenModelImageID(c *gc.C) {
	defer s.setupMocks(c).Finish()
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{
		Arch:    strptr("arm64"),
		ImageID: strptr("ubuntu-bf2"),
	}, nil)

	_, _, err := s.getValidator().resolveCharm(curl, origin, false, false, constraints.Value{})
	c.Assert(err, gc.ErrorMatches, `base must be explicitly provided when image-id constraint is used`)
}

func (s *validatorSuite) TestCreateOrigin(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))

	arg := params.DeployFromRepositoryArg{
		CharmName: "testcharm",
		Revision:  intptr(7),
	}
	curl, origin, defaultBase, err := s.getValidator().createOrigin(arg)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(curl, gc.DeepEquals, charm.MustParseURL("ch:testcharm-7"))
	c.Assert(origin, gc.DeepEquals, corecharm.Origin{
		Source:   "charm-hub",
		Revision: intptr(7),
		Channel:  &corecharm.DefaultChannel,
		Platform: corecharm.Platform{Architecture: "amd64"},
	})
	c.Assert(defaultBase, jc.IsFalse)
}

func (s *validatorSuite) TestCreateOriginChannel(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))

	arg := params.DeployFromRepositoryArg{
		CharmName: "testcharm",
		Revision:  intptr(7),
		Channel:   strptr("yoga/candidate"),
	}
	curl, origin, defaultBase, err := s.getValidator().createOrigin(arg)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(curl, gc.DeepEquals, charm.MustParseURL("ch:testcharm-7"))
	expectedChannel := corecharm.MustParseChannel("yoga/candidate")
	c.Assert(origin, gc.DeepEquals, corecharm.Origin{
		Source:   "charm-hub",
		Revision: intptr(7),
		Channel:  &expectedChannel,
		Platform: corecharm.Platform{Architecture: "amd64"},
	})
	c.Assert(defaultBase, jc.IsFalse)
}

func (s *validatorSuite) TestGetCharm(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.expectSimpleValidate()
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	// resolveCharm
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	// getCharm
	expMeta := &charm.Meta{
		Name: "test-charm",
	}
	expManifest := new(charm.Manifest)
	expConfig := new(charm.Config)
	essMeta := corecharm.EssentialMetadata{
		Meta:           expMeta,
		Manifest:       expManifest,
		Config:         expConfig,
		ResolvedOrigin: resolvedOrigin,
	}
	s.repo.EXPECT().GetEssentialMetadata(corecharm.MetadataRequest{
		CharmURL: resultURL,
		Origin:   resolvedOrigin,
	}).Return([]corecharm.EssentialMetadata{essMeta}, nil)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testcharm",
	}
	obtainedURL, obtainedOrigin, obtainedCharm, err := s.getValidator().getCharm(arg)
	c.Assert(err, gc.HasLen, 0)
	c.Assert(obtainedOrigin, gc.DeepEquals, resolvedOrigin)
	c.Assert(obtainedCharm, gc.DeepEquals, corecharm.NewCharmInfoAdapter(essMeta))
	c.Assert(obtainedURL.String(), gc.Equals, resultURL.String())
}

func (s *validatorSuite) TestDeducePlatformSimple(c *gc.C) {
	defer s.setupMocks(c).Finish()
	//model constraint default
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("amd64")}, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))

	arg := params.DeployFromRepositoryArg{CharmName: "testme"}
	plat, usedModelDefaultBase, err := s.getValidator().deducePlatform(arg)
	c.Assert(err, gc.IsNil)
	c.Assert(usedModelDefaultBase, jc.IsFalse)
	c.Assert(plat, gc.DeepEquals, corecharm.Platform{Architecture: "amd64"})
}

func (s *validatorSuite) TestDeducePlatformArgArchBase(c *gc.C) {
	defer s.setupMocks(c).Finish()

	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
		Cons:      constraints.Value{Arch: strptr("arm64")},
		Base: &params.Base{
			Name:    "ubuntu",
			Channel: "22.10",
		},
	}
	plat, usedModelDefaultBase, err := s.getValidator().deducePlatform(arg)
	c.Assert(err, gc.IsNil)
	c.Assert(usedModelDefaultBase, jc.IsFalse)
	c.Assert(plat, gc.DeepEquals, corecharm.Platform{
		Architecture: "arm64",
		OS:           "ubuntu",
		Channel:      "22.10",
	})
}

func (s *validatorSuite) TestDeducePlatformModelDefaultBase(c *gc.C) {
	defer s.setupMocks(c).Finish()
	//model constraint default
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	sConfig := coretesting.FakeConfig()
	sConfig = sConfig.Merge(coretesting.Attrs{
		"default-base": "ubuntu@22.04",
	})
	cfg, err := config.New(config.NoDefaults, sConfig)
	c.Assert(err, jc.ErrorIsNil)
	s.model.EXPECT().Config().Return(cfg, nil)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
	}
	plat, usedModelDefaultBase, err := s.getValidator().deducePlatform(arg)
	c.Assert(err, gc.IsNil)
	c.Assert(usedModelDefaultBase, jc.IsTrue)
	c.Assert(plat, gc.DeepEquals, corecharm.Platform{
		Architecture: "amd64",
		OS:           "ubuntu",
		Channel:      "22.04/stable",
	})
}

func (s *validatorSuite) TestDeducePlatformPlacementSimpleFound(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	s.state.EXPECT().Machine("0").Return(s.machine, nil)
	s.machine.EXPECT().Base().Return(state.Base{
		OS:      "ubuntu",
		Channel: "18.04",
	})
	hwc := &instance.HardwareCharacteristics{Arch: strptr("arm64")}
	s.machine.EXPECT().HardwareCharacteristics().Return(hwc, nil)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
		Placement: []*instance.Placement{{
			Directive: "0",
		}},
	}
	plat, usedModelDefaultBase, err := s.getValidator().deducePlatform(arg)
	c.Assert(err, gc.IsNil)
	c.Assert(usedModelDefaultBase, jc.IsFalse)
	c.Assert(plat, gc.DeepEquals, corecharm.Platform{
		Architecture: "arm64",
		OS:           "ubuntu",
		Channel:      "18.04",
	})
}

func (s *validatorSuite) TestDeducePlatformPlacementSimpleNotFound(c *gc.C) {
	defer s.setupMocks(c).Finish()
	//model constraint default
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("amd64")}, nil)
	s.model.EXPECT().Config().Return(config.New(config.UseDefaults, coretesting.FakeConfig()))
	s.state.EXPECT().Machine("0/lxd/0").Return(nil, errors.NotFoundf("machine 0/lxd/0 not found"))

	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
		Placement: []*instance.Placement{{
			Directive: "0/lxd/0",
		}},
	}
	plat, usedModelDefaultBase, err := s.getValidator().deducePlatform(arg)
	c.Assert(err, gc.IsNil)
	c.Assert(usedModelDefaultBase, jc.IsFalse)
	c.Assert(plat, gc.DeepEquals, corecharm.Platform{Architecture: "amd64"})
}

func (s *validatorSuite) TestDeducePlatformPlacementMutipleMatch(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	s.state.EXPECT().Machine(gomock.Any()).Return(s.machine, nil).Times(3)
	s.machine.EXPECT().Base().Return(state.Base{
		OS:      "ubuntu",
		Channel: "18.04",
	}).Times(3)
	hwc := &instance.HardwareCharacteristics{Arch: strptr("arm64")}
	s.machine.EXPECT().HardwareCharacteristics().Return(hwc, nil).Times(3)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
		Placement: []*instance.Placement{
			{Directive: "0"},
			{Directive: "1"},
			{Directive: "3"},
		},
	}
	plat, usedModelDefaultBase, err := s.getValidator().deducePlatform(arg)
	c.Assert(err, gc.IsNil)
	c.Assert(usedModelDefaultBase, jc.IsFalse)
	c.Assert(plat, gc.DeepEquals, corecharm.Platform{
		Architecture: "arm64",
		OS:           "ubuntu",
		Channel:      "18.04",
	})
}

func (s *validatorSuite) TestDeducePlatformPlacementMutipleMatchFail(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{}, nil)
	s.state.EXPECT().Machine(gomock.Any()).Return(s.machine, nil).AnyTimes()
	s.machine.EXPECT().Base().Return(
		state.Base{
			OS:      "ubuntu",
			Channel: "18.04",
		}).AnyTimes()
	gomock.InOrder(
		s.machine.EXPECT().HardwareCharacteristics().Return(
			&instance.HardwareCharacteristics{Arch: strptr("arm64")},
			nil),
		s.machine.EXPECT().HardwareCharacteristics().Return(
			&instance.HardwareCharacteristics{Arch: strptr("amd64")},
			nil),
	)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
		Placement: []*instance.Placement{
			{Directive: "0"},
			{Directive: "1"},
		},
	}
	_, _, err := s.getValidator().deducePlatform(arg)
	c.Assert(errors.Is(err, errors.BadRequest), jc.IsTrue, gc.Commentf("%+v", err))
}

var configYaml = `
testme:
  optionOne: one
  optionTwo: 8
`[1:]

func (s *validatorSuite) TestAppCharmSettings(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.model.EXPECT().Type().Return(state.ModelTypeIAAS)

	cfg := charm.NewConfig()
	cfg.Options = map[string]charm.Option{
		"optionOne": {
			Type:        "string",
			Description: "option one",
		},
		"optionTwo": {
			Type:        "int",
			Description: "option two",
		},
	}

	appCfgSchema, _, err := applicationConfigSchema(state.ModelTypeIAAS)
	c.Assert(err, jc.ErrorIsNil)

	expectedAppConfig, err := coreconfig.NewConfig(map[string]interface{}{"trust": true}, appCfgSchema, nil)
	c.Assert(err, jc.ErrorIsNil)

	appConfig, charmConfig, err := s.getValidator().appCharmSettings("testme", true, cfg, configYaml)
	c.Assert(err, jc.ErrorIsNil)
	c.Check(appConfig, gc.DeepEquals, expectedAppConfig)
	c.Assert(charmConfig["optionOne"], gc.DeepEquals, "one")
	c.Assert(charmConfig["optionTwo"], gc.DeepEquals, int64(8))
}

func (s *validatorSuite) TestCaasDeployFromRepositoryValidator(c *gc.C) {
	defer s.setupMocks(c).Finish()
	s.expectSimpleValidate()
	// resolveCharm
	curl := charm.MustParseURL("testcharm")
	resultURL := charm.MustParseURL("ch:amd64/jammy/testcharm-4")
	origin := corecharm.Origin{
		Source:   "charm-hub",
		Channel:  &charm.Channel{Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64"},
	}
	resolvedOrigin := corecharm.Origin{
		Source:   "charm-hub",
		Type:     "charm",
		Channel:  &charm.Channel{Track: "default", Risk: "stable"},
		Platform: corecharm.Platform{Architecture: "amd64", OS: "ubuntu", Channel: "22.04/stable"},
		Revision: intptr(4),
	}
	supportedSeries := []string{"jammy", "focal"}
	s.repo.EXPECT().ResolveWithPreferredChannel(curl, origin).Return(resultURL, resolvedOrigin, supportedSeries, nil)
	// getCharm
	expMeta := &charm.Meta{
		Name: "test-charm",
	}
	expManifest := new(charm.Manifest)
	expConfig := new(charm.Config)
	essMeta := corecharm.EssentialMetadata{
		Meta:           expMeta,
		Manifest:       expManifest,
		Config:         expConfig,
		ResolvedOrigin: resolvedOrigin,
	}
	s.repo.EXPECT().GetEssentialMetadata(corecharm.MetadataRequest{
		CharmURL: resultURL,
		Origin:   resolvedOrigin,
	}).Return([]corecharm.EssentialMetadata{essMeta}, nil)
	s.state.EXPECT().ModelConstraints().Return(constraints.Value{Arch: strptr("arm64")}, nil)

	arg := params.DeployFromRepositoryArg{
		CharmName: "testcharm",
	}

	obtainedDT, errs := s.caasDeployFromRepositoryValidator(c).ValidateArg(arg)
	c.Assert(errs, gc.HasLen, 0)
	c.Logf("%s", pretty.Sprint(obtainedDT))
	c.Assert(obtainedDT, gc.DeepEquals, deployTemplate{
		applicationName: "test-charm",
		charm:           corecharm.NewCharmInfoAdapter(essMeta),
		charmURL:        resultURL,
		numUnits:        1,
		origin:          resolvedOrigin,
	})
}

func (s *validatorSuite) setupMocks(c *gc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)
	s.bindings = NewMockBindings(ctrl)
	s.machine = NewMockMachine(ctrl)
	s.model = NewMockModel(ctrl)
	s.repo = NewMockRepository(ctrl)
	s.repoFactory = NewMockRepositoryFactory(ctrl)
	s.state = NewMockDeployFromRepositoryState(ctrl)
	return ctrl
}

func (s *validatorSuite) getValidator() *deployFromRepositoryValidator {
	s.repoFactory.EXPECT().GetCharmRepository(gomock.Any()).Return(s.repo, nil).AnyTimes()
	return &deployFromRepositoryValidator{
		model:       s.model,
		state:       s.state,
		repoFactory: s.repoFactory,
		newStateBindings: func(st state.EndpointBinding, givenMap map[string]string) (Bindings, error) {
			return s.bindings, nil
		},
	}
}

func (s *validatorSuite) caasDeployFromRepositoryValidator(c *gc.C) caasDeployFromRepositoryValidator {
	return caasDeployFromRepositoryValidator{
		validator: s.getValidator(),
		caasPrecheckFunc: func(dt deployTemplate) error {
			// Do a quick check to ensure the expected deployTemplate
			// has been passed.
			c.Assert(dt.applicationName, gc.Equals, "test-charm")
			return nil
		},
	}
}

func strptr(s string) *string {
	return &s
}

func intptr(i int) *int {
	return &i
}

type deployRepositorySuite struct {
	application *MockApplication
	charm       *MockCharm
	state       *MockDeployFromRepositoryState
	validator   *MockDeployFromRepositoryValidator
}

func (s *deployRepositorySuite) TestDeployFromRepositoryAPI(c *gc.C) {
	defer s.setupMocks(c).Finish()
	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
	}
	template := deployTemplate{
		applicationName: "metadata-name",
		charm:           corecharm.NewCharmInfoAdapter(corecharm.EssentialMetadata{}),
		charmURL:        charm.MustParseURL("ch:amd64/jammy/testme-5"),
		endpoints:       map[string]string{"to": "from"},
		numUnits:        1,
		origin: corecharm.Origin{
			Source:   "charm-hub",
			Revision: intptr(5),
			Channel:  &charm.Channel{Risk: "stable"},
			Platform: corecharm.MustParsePlatform("amd64/ubuntu/22.04"),
		},
		placement: []*instance.Placement{{Directive: "0", Scope: instance.MachineScope}},
	}
	s.validator.EXPECT().ValidateArg(arg).Return(template, nil)
	info := state.CharmInfo{
		Charm: template.charm,
		ID:    charm.MustParseURL("ch:amd64/jammy/testme-5"),
	}

	s.charm.EXPECT().Meta().Return(&charm.Meta{Resources: nil})

	s.state.EXPECT().AddCharmMetadata(info).Return(s.charm, nil)

	addAppArgs := state.AddApplicationArgs{
		Name: "metadata-name",
		// the app.Charm is casted into a state.Charm in the code
		// we mock it separately here (s.charm above), the test works
		// thanks to the addApplicationArgsMatcher used below
		Charm: &state.Charm{},
		CharmOrigin: &state.CharmOrigin{
			Source:   "charm-hub",
			Revision: intptr(5),
			Channel: &state.Channel{
				Risk: "stable",
			},
			Platform: &state.Platform{
				Architecture: "amd64",
				OS:           "ubuntu",
				Channel:      "22.04",
			},
		},
		EndpointBindings: map[string]string{"to": "from"},
		NumUnits:         1,
		Placement:        []*instance.Placement{{Directive: "0", Scope: instance.MachineScope}},
		Resources:        map[string]string{},
		Storage:          map[string]state.StorageConstraints{},
	}
	s.state.EXPECT().AddApplication(addApplicationArgsMatcher{c: c, expectedArgs: addAppArgs}).Return(s.application, nil)

	deployFromRepositoryAPI := s.getDeployFromRepositoryAPI()

	obtainedInfo, resources, errs := deployFromRepositoryAPI.DeployFromRepository(arg)
	c.Assert(errs, gc.HasLen, 0)
	c.Assert(resources, gc.HasLen, 0)
	c.Assert(obtainedInfo, gc.DeepEquals, params.DeployFromRepositoryInfo{
		Architecture:     "amd64",
		Base:             params.Base{Name: "ubuntu", Channel: "22.04"},
		Channel:          "stable",
		EffectiveChannel: nil,
		Name:             "metadata-name",
		Revision:         5,
	})
}

// The reason for this matcher is that the AddApplicationArgs.Charm is
// obtained by casting application.Charm into a state.Charm, but we
// can't do that cast with a MockCharm
type addApplicationArgsMatcher struct {
	c            *gc.C
	expectedArgs state.AddApplicationArgs
}

func (m addApplicationArgsMatcher) String() string {
	return "match AddApplicationArgs"
}

func (m addApplicationArgsMatcher) Matches(x interface{}) bool {

	oA, ok := x.(state.AddApplicationArgs)
	if !ok {
		return false
	}

	eA := m.expectedArgs
	// Check everything but the Charm
	m.c.Assert(oA.Name, gc.DeepEquals, eA.Name)
	m.c.Assert(oA.ApplicationConfig, gc.DeepEquals, eA.ApplicationConfig)
	m.c.Assert(oA.NumUnits, gc.DeepEquals, eA.NumUnits)
	m.c.Assert(oA.Constraints, gc.DeepEquals, eA.Constraints)
	m.c.Assert(oA.Storage, gc.DeepEquals, eA.Storage)
	m.c.Assert(oA.Devices, gc.DeepEquals, eA.Devices)
	m.c.Assert(eA.AttachStorage, gc.DeepEquals, eA.AttachStorage)
	m.c.Assert(oA.EndpointBindings, gc.DeepEquals, eA.EndpointBindings)
	m.c.Assert(oA.CharmConfig, gc.DeepEquals, eA.CharmConfig)
	m.c.Assert(oA.Placement, gc.DeepEquals, eA.Placement)
	m.c.Assert(oA.Resources, gc.DeepEquals, eA.Resources)
	return true
}

func (s *deployRepositorySuite) TestAddPendingResourcesForDeployFromRepositoryAPI(c *gc.C) {
	defer s.setupMocks(c).Finish()
	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
	}
	template := deployTemplate{
		applicationName: "metadata-name",
		charm:           corecharm.NewCharmInfoAdapter(corecharm.EssentialMetadata{}),
		charmURL:        charm.MustParseURL("ch:amd64/jammy/testme-5"),
		endpoints:       map[string]string{"to": "from"},
		numUnits:        1,
		origin: corecharm.Origin{
			Source:   "charm-hub",
			Revision: intptr(5),
			Channel:  &charm.Channel{Risk: "stable"},
			Platform: corecharm.MustParsePlatform("amd64/ubuntu/22.04"),
		},
		placement: []*instance.Placement{{Directive: "0", Scope: instance.MachineScope}},
		resources: map[string]string{"foo-file": "bar"},
	}
	s.validator.EXPECT().ValidateArg(arg).Return(template, nil)
	info := state.CharmInfo{
		Charm: template.charm,
		ID:    charm.MustParseURL("ch:amd64/jammy/testme-5"),
	}

	s.state.EXPECT().AddCharmMetadata(info).Return(s.charm, nil)

	meta := resource.Meta{
		Name:        "foo-resource",
		Type:        resource.TypeFile,
		Path:        "foo.txt",
		Description: "bar",
	}
	r := resource.Resource{
		Meta:   meta,
		Origin: resource.OriginUpload,
	}
	s.charm.EXPECT().Meta().Return(&charm.Meta{
		Resources: map[string]resource.Meta{
			"foo-file": meta,
		}})

	s.state.EXPECT().AddPendingResource("metadata-name", r).Return("3", nil)

	addAppArgs := state.AddApplicationArgs{
		Name: "metadata-name",
		// the app.Charm is casted into a state.Charm in the code
		// we mock it separately here (s.charm above), the test works
		// thanks to the addApplicationArgsMatcher used below
		Charm: &state.Charm{},
		CharmOrigin: &state.CharmOrigin{
			Source:   "charm-hub",
			Revision: intptr(5),
			Channel: &state.Channel{
				Risk: "stable",
			},
			Platform: &state.Platform{
				Architecture: "amd64",
				OS:           "ubuntu",
				Channel:      "22.04",
			},
		},
		EndpointBindings: map[string]string{"to": "from"},
		NumUnits:         1,
		Placement:        []*instance.Placement{{Directive: "0", Scope: instance.MachineScope}},
		Resources:        map[string]string{"foo-file": "3"},
		Storage:          map[string]state.StorageConstraints{},
	}
	s.state.EXPECT().AddApplication(addApplicationArgsMatcher{c: c, expectedArgs: addAppArgs}).Return(s.application, nil)

	deployFromRepositoryAPI := s.getDeployFromRepositoryAPI()

	obtainedInfo, resources, errs := deployFromRepositoryAPI.DeployFromRepository(arg)
	c.Assert(errs, gc.HasLen, 0)
	c.Assert(resources, gc.HasLen, 1)
	c.Assert(obtainedInfo, gc.DeepEquals, params.DeployFromRepositoryInfo{
		Architecture:     "amd64",
		Base:             params.Base{Name: "ubuntu", Channel: "22.04"},
		Channel:          "stable",
		EffectiveChannel: nil,
		Name:             "metadata-name",
		Revision:         5,
	})

	pendUp := &params.PendingResourceUpload{
		Name:     "foo-resource",
		Type:     "file",
		Filename: "bar",
	}
	c.Assert(resources, gc.DeepEquals, []*params.PendingResourceUpload{pendUp})
}

func (s *deployRepositorySuite) TestRemovePendingResourcesWhenDeployErrors(c *gc.C) {
	defer s.setupMocks(c).Finish()
	arg := params.DeployFromRepositoryArg{
		CharmName: "testme",
	}
	template := deployTemplate{
		applicationName: "metadata-name",
		charm:           corecharm.NewCharmInfoAdapter(corecharm.EssentialMetadata{}),
		charmURL:        charm.MustParseURL("ch:amd64/jammy/testme-5"),
		endpoints:       map[string]string{"to": "from"},
		numUnits:        1,
		origin: corecharm.Origin{
			Source:   "charm-hub",
			Revision: intptr(5),
			Channel:  &charm.Channel{Risk: "stable"},
			Platform: corecharm.MustParsePlatform("amd64/ubuntu/22.04"),
		},
		placement: []*instance.Placement{{Directive: "0", Scope: instance.MachineScope}},
		resources: map[string]string{"foo-file": "bar"},
	}
	s.validator.EXPECT().ValidateArg(arg).Return(template, nil)
	info := state.CharmInfo{
		Charm: template.charm,
		ID:    charm.MustParseURL("ch:amd64/jammy/testme-5"),
	}

	s.state.EXPECT().AddCharmMetadata(info).Return(s.charm, nil)

	meta := resource.Meta{
		Name:        "foo-resource",
		Type:        resource.TypeFile,
		Path:        "foo.txt",
		Description: "bar",
	}
	r := resource.Resource{
		Meta:   meta,
		Origin: resource.OriginUpload,
	}
	s.charm.EXPECT().Meta().Return(&charm.Meta{
		Resources: map[string]resource.Meta{
			"foo-file": meta,
		}})

	s.state.EXPECT().AddPendingResource("metadata-name", r).Return("3", nil)

	addAppArgs := state.AddApplicationArgs{
		Name: "metadata-name",
		// the app.Charm is casted into a state.Charm in the code
		// we mock it separately here (s.charm above), the test works
		// thanks to the addApplicationArgsMatcher used below
		Charm: &state.Charm{},
		CharmOrigin: &state.CharmOrigin{
			Source:   "charm-hub",
			Revision: intptr(5),
			Channel: &state.Channel{
				Risk: "stable",
			},
			Platform: &state.Platform{
				Architecture: "amd64",
				OS:           "ubuntu",
				Channel:      "22.04",
			},
		},
		EndpointBindings: map[string]string{"to": "from"},
		NumUnits:         1,
		Placement:        []*instance.Placement{{Directive: "0", Scope: instance.MachineScope}},
		Resources:        map[string]string{"foo-file": "3"},
		Storage:          map[string]state.StorageConstraints{},
	}

	s.state.EXPECT().RemovePendingResources("metadata-name", map[string]string{"foo-file": "3"})

	s.state.EXPECT().AddApplication(addApplicationArgsMatcher{c: c, expectedArgs: addAppArgs}).Return(s.application,
		errors.New("fail"))

	deployFromRepositoryAPI := s.getDeployFromRepositoryAPI()

	obtainedInfo, resources, errs := deployFromRepositoryAPI.DeployFromRepository(arg)
	c.Assert(errs, gc.HasLen, 1)
	c.Assert(resources, gc.HasLen, 0)
	c.Assert(obtainedInfo, gc.DeepEquals, params.DeployFromRepositoryInfo{})
}

func (s *deployRepositorySuite) getDeployFromRepositoryAPI() *DeployFromRepositoryAPI {
	return &DeployFromRepositoryAPI{
		state:      s.state,
		validator:  s.validator,
		stateCharm: func(Charm) *state.Charm { return nil },
	}
}

func (s *deployRepositorySuite) setupMocks(c *gc.C) *gomock.Controller {
	ctrl := gomock.NewController(c)
	s.charm = NewMockCharm(ctrl)
	s.state = NewMockDeployFromRepositoryState(ctrl)
	s.validator = NewMockDeployFromRepositoryValidator(ctrl)
	return ctrl
}
