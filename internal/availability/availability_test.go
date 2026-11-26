package availability_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/availability"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

type AvailabilitySuite struct {
	suite.Suite
}

func TestAvailabilitySuite(t *testing.T) {
	suite.Run(t, new(AvailabilitySuite))
}

func delivered(quantity int) repository.Switch {
	return repository.Switch{ID: "sw1", Purchases: repository.BulkPurchases{
		{Quantity: new(quantity), OrderStatus: new("Delivered")},
	}}
}

func build(keyboard string, date *string, switchCount int) repository.Build {
	return repository.Build{
		Keyboard:  keyboard,
		BuildDate: date,
		Switches:  []repository.BuildSwitchEntry{{Switch: "sw1", Count: switchCount}},
	}
}

func (s *AvailabilitySuite) TestNoBuilds_AllDeliveredAvailable() {
	s.Equal(90, availability.Usage{}.SwitchQuantity(delivered(90)))
}

func (s *AvailabilitySuite) TestNotDelivered_NotAvailable() {
	sw := repository.Switch{ID: "sw1", Purchases: repository.BulkPurchases{
		{Quantity: new(90), OrderStatus: new("Shipped")},
	}}

	s.Equal(0, availability.Usage{}.SwitchQuantity(sw))
}

func (s *AvailabilitySuite) TestCurrentBuilds_SubtractedAcrossKeyboards() {
	u := availability.FromBuilds([]repository.Build{
		build("kb1", new("2026-01-01"), 65),
		build("kb2", nil, 20),
	})

	s.Equal(25, u.SwitchQuantity(delivered(110)))
}

func (s *AvailabilitySuite) TestOlderBuildOfSameKeyboard_NotCounted() {
	u := availability.FromBuilds([]repository.Build{
		build("kb1", new("2026-01-01"), 65),
		build("kb1", new("2026-06-01"), 30),
		build("kb1", nil, 50),
	})

	s.Equal(60, u.SwitchQuantity(delivered(90)))
}

func (s *AvailabilitySuite) TestSameDate_FirstBuildIsCurrent() {
	u := availability.FromBuilds([]repository.Build{
		build("kb1", new("2026-01-01"), 65),
		build("kb1", new("2026-01-01"), 30),
	})

	s.Equal(25, u.SwitchQuantity(delivered(90)))
}

func (s *AvailabilitySuite) TestMoreUsedThanDelivered_FloorsAtZero() {
	u := availability.FromBuilds([]repository.Build{build("kb1", nil, 100)})

	s.Equal(0, u.SwitchQuantity(delivered(90)))
}

func (s *AvailabilitySuite) TestLoad_ReadsEveryPageAtEveryVisibility() {
	builds := mocks.NewMockBuildRepository(s.T())
	all := []repository.Visibility{repository.VisibilityPublic, repository.VisibilityAuthenticated, repository.VisibilityPrivate}
	builds.EXPECT().List(mock.Anything, "alice", all, "", 100, "").
		Return([]repository.Build{build("kb1", nil, 10)}, "next", nil)
	builds.EXPECT().List(mock.Anything, "alice", all, "", 100, "next").
		Return([]repository.Build{build("kb2", nil, 20)}, "", nil)

	u, err := availability.Load(context.Background(), builds, "alice")

	s.Require().NoError(err)
	s.Equal(60, u.SwitchQuantity(delivered(90)))
}

func (s *AvailabilitySuite) TestLoad_ListError_ReturnsError() {
	builds := mocks.NewMockBuildRepository(s.T())
	builds.EXPECT().List(mock.Anything, "alice", mock.Anything, "", 100, "").
		Return(nil, "", errors.New("boom"))

	_, err := availability.Load(context.Background(), builds, "alice")

	s.Require().Error(err)
}

func kit(id, name, status string) repository.KeycapKit {
	k := repository.KeycapKit{KitID: id, Name: name}
	if status != "" {
		k.Purchase.OrderStatus = new(status)
	}
	return k
}

func set(primary *string, kits ...repository.KeycapKit) repository.KeycapSet {
	ks := repository.KeycapSet{ID: "ks1", PrimaryKitID: primary, Kits: map[string]repository.KeycapKit{}}
	for _, k := range kits {
		ks.Kits[k.KitID] = k
	}
	return ks
}

func buildWithKit(date *string, kitID string) repository.Build {
	return repository.Build{
		Keyboard:   "kb1",
		BuildDate:  date,
		KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks1", Kit: kitID}},
	}
}

func (s *AvailabilitySuite) TestKeycapSet_PrimaryKitDecides() {
	ks := set(new("k2"), kit("k1", "Base", "Delivered"), kit("k2", "Alphas", "Shipped"))

	s.False(availability.Usage{}.KeycapSetAvailable(ks))
}

func (s *AvailabilitySuite) TestKeycapSet_DanglingPrimary_FallsBackToBaseName() {
	ks := set(new("gone"), kit("k1", "Base", "Delivered"), kit("k2", "Novelties", "Shipped"))

	s.True(availability.Usage{}.KeycapSetAvailable(ks))
}

func (s *AvailabilitySuite) TestKeycapSet_OnlyKitIsBase() {
	s.True(availability.Usage{}.KeycapSetAvailable(set(nil, kit("k1", "Alphas", "Delivered"))))
}

func (s *AvailabilitySuite) TestKeycapSet_DeliveredBaseWithAddOnStillShipping_Available() {
	ks := set(nil, kit("k1", "BASE kit", "Delivered"), kit("k2", "Spacebars", "Shipped"))

	s.True(availability.Usage{}.KeycapSetAvailable(ks))
}

func (s *AvailabilitySuite) TestKeycapSet_NoBaseKit_NotAvailable() {
	ks := set(nil, kit("k1", "Alphas", "Delivered"), kit("k2", "Mods", "Delivered"))

	s.False(availability.Usage{}.KeycapSetAvailable(ks))
}

func (s *AvailabilitySuite) TestKeycapSet_NoKits_NotAvailable() {
	s.False(availability.Usage{}.KeycapSetAvailable(set(nil)))
}

func (s *AvailabilitySuite) TestKeycapSet_BaseOnCurrentBuild_NotAvailable() {
	ks := set(nil, kit("k1", "Base", "Delivered"))
	u := availability.FromBuilds([]repository.Build{buildWithKit(nil, "k1")})

	s.False(u.KeycapSetAvailable(ks))
}

func (s *AvailabilitySuite) TestKeycapSet_BaseOnlyOnOlderBuild_Available() {
	ks := set(nil, kit("k1", "Base", "Delivered"))
	u := availability.FromBuilds([]repository.Build{
		buildWithKit(new("2026-01-01"), "k1"),
		buildWithKit(new("2026-06-01"), "other"),
	})

	s.True(u.KeycapSetAvailable(ks))
}

func (s *AvailabilitySuite) TestKeycapSet_OneOfTwoBasesFree_Available() {
	ks := set(nil, kit("k1", "Base", "Delivered"), kit("k2", "Base (second)", "Delivered"))
	u := availability.FromBuilds([]repository.Build{buildWithKit(nil, "k1")})

	s.True(u.KeycapSetAvailable(ks))
}
