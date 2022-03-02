//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package processors

import (
	"context"
	"testing"

	"github.com/stellar/go/gxdr"
	"github.com/stellar/go/ingest"
	"github.com/stellar/go/randxdr"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/test"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
	"github.com/stellar/go/xdr"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func TestFuzzOffers(t *testing.T) {
	tt := test.Start(t)
	test.ResetHorizonDB(t, tt.HorizonDB)
	q := &history.Q{&db.Session{DB: tt.HorizonDB}}
	pp := NewOffersProcessor(q, 10)
	gen := randxdr.NewGenerator()

	var changes []xdr.LedgerEntryChange
	for i := 0; i < 1000; i++ {
		change := xdr.LedgerEntryChange{}
		shape := &gxdr.LedgerEntryChange{}
		gen.Next(
			shape,
			[]randxdr.Preset{
				{randxdr.FieldEquals("type"), randxdr.SetU32(gxdr.LEDGER_ENTRY_CREATED.GetU32())},
				// the offers postgres table is configured with some database constraints which validate the following
				// fields:
				{randxdr.FieldEquals("created.lastModifiedLedgerSeq"), randxdr.SetPositiveNum32()},
				{randxdr.FieldEquals("created.data.offer.amount"), randxdr.SetPositiveNum64()},
				{randxdr.FieldEquals("created.data.offer.price.n"), randxdr.SetPositiveNum32()},
				{randxdr.FieldEquals("created.data.offer.price.d"), randxdr.SetPositiveNum32()},
			},
		)
		tt.Assert.NoError(gxdr.Convert(shape, &change))
		changes = append(changes, change)
	}

	for _, change := range ingest.GetChangesFromLedgerEntryChanges(changes) {
		tt.Assert.NoError(pp.ProcessChange(tt.Ctx, change))
	}

	tt.Assert.NoError(pp.Commit(tt.Ctx))
}

func TestOffersProcessorTestSuiteState(t *testing.T) {
	suite.Run(t, new(OffersProcessorTestSuiteState))
}

type OffersProcessorTestSuiteState struct {
	suite.Suite
	ctx       context.Context
	processor *OffersProcessor
	mockQ     *history.MockQOffers
	sequence  uint32
}

func (s *OffersProcessorTestSuiteState) SetupTest() {
	s.ctx = context.Background()
	s.mockQ = &history.MockQOffers{}

	s.sequence = 456
	s.processor = NewOffersProcessor(s.mockQ, s.sequence)
}

func (s *OffersProcessorTestSuiteState) TearDownTest() {
	s.mockQ.On("CompactOffers", s.ctx, s.sequence-100).Return(int64(0), nil).Once()
	s.Assert().NoError(s.processor.Commit(s.ctx))

	s.mockQ.AssertExpectations(s.T())
}

func (s *OffersProcessorTestSuiteState) TestCreateOffer() {
	offer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		OfferId:  xdr.Int64(1),
		Price:    xdr.Price{1, 2},
	}
	lastModifiedLedgerSeq := xdr.Uint32(123)
	entry := xdr.LedgerEntry{
		Data: xdr.LedgerEntryData{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &offer,
		},
		LastModifiedLedgerSeq: lastModifiedLedgerSeq,
	}

	s.mockQ.On("UpsertOffers", s.ctx, []history.Offer{
		{
			SellerID:           "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			OfferID:            1,
			Pricen:             int32(1),
			Priced:             int32(2),
			Price:              float64(0.5),
			LastModifiedLedger: uint32(lastModifiedLedgerSeq),
		},
	}).Return(nil).Once()

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre:  nil,
		Post: &entry,
	})
	s.Assert().NoError(err)
}

func TestOffersProcessorTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(OffersProcessorTestSuiteLedger))
}

type OffersProcessorTestSuiteLedger struct {
	suite.Suite
	ctx       context.Context
	processor *OffersProcessor
	mockQ     *history.MockQOffers
	sequence  uint32
}

func (s *OffersProcessorTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.mockQ = &history.MockQOffers{}

	s.sequence = 456
	s.processor = NewOffersProcessor(s.mockQ, s.sequence)
}

func (s *OffersProcessorTestSuiteLedger) TearDownTest() {
	s.mockQ.AssertExpectations(s.T())
}

func (s *OffersProcessorTestSuiteLedger) setupInsertOffer() {
	// should be ignored because it's not an offer type
	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:  xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Thresholds: [4]byte{1, 1, 1, 1},
				},
			},
		},
	})
	s.Assert().NoError(err)

	// add offer
	offer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		OfferId:  xdr.Int64(2),
		Price:    xdr.Price{1, 2},
	}
	lastModifiedLedgerSeq := xdr.Uint32(1234)

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:  xdr.LedgerEntryTypeOffer,
				Offer: &offer,
			},
		},
	})
	s.Assert().NoError(err)

	updatedOffer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		OfferId:  xdr.Int64(2),
		Price:    xdr.Price{1, 6},
	}

	updatedEntry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		Data: xdr.LedgerEntryData{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &updatedOffer,
		},
	}

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq - 1,
			Data: xdr.LedgerEntryData{
				Type:  xdr.LedgerEntryTypeOffer,
				Offer: &offer,
			},
		},
		Post: &updatedEntry,
	})
	s.Assert().NoError(err)

	// We use LedgerEntryChangesCache so all changes are squashed
	s.mockQ.On("UpsertOffers", s.ctx, []history.Offer{
		{
			SellerID:           "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			OfferID:            2,
			Pricen:             int32(1),
			Priced:             int32(6),
			Price:              float64(1) / float64(6),
			LastModifiedLedger: uint32(lastModifiedLedgerSeq),
		},
	}).Return(nil).Once()
}

func (s *OffersProcessorTestSuiteLedger) TestInsertOffer() {
	s.setupInsertOffer()
	s.mockQ.On("CompactOffers", s.ctx, s.sequence-100).Return(int64(0), nil).Once()
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *OffersProcessorTestSuiteLedger) TestSkipCompactionIfSequenceEqualsWindow() {
	s.processor.sequence = offerCompactionWindow
	s.setupInsertOffer()
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *OffersProcessorTestSuiteLedger) TestSkipCompactionIfSequenceLessThanWindow() {
	s.processor.sequence = offerCompactionWindow - 1
	s.setupInsertOffer()
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *OffersProcessorTestSuiteLedger) TestCompactionError() {
	s.setupInsertOffer()
	s.mockQ.On("CompactOffers", s.ctx, s.sequence-100).
		Return(int64(0), errors.New("compaction error")).Once()
	s.Assert().EqualError(s.processor.Commit(s.ctx), "could not compact offers: compaction error")
}

func (s *OffersProcessorTestSuiteLedger) TestUpsertManyOffers() {
	lastModifiedLedgerSeq := xdr.Uint32(1234)

	offer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		OfferId:  xdr.Int64(2),
		Price:    xdr.Price{1, 2},
	}
	updatedOffer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		OfferId:  xdr.Int64(2),
		Price:    xdr.Price{1, 6},
	}

	anotherOffer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GDMUVYVYPYZYBDXNJWKFT3X2GCZCICTL3GSVP6AWBGB4ZZG7ZRDA746P"),
		OfferId:  xdr.Int64(3),
		Price:    xdr.Price{2, 3},
	}

	updatedEntry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		Data: xdr.LedgerEntryData{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &updatedOffer,
		},
	}

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq - 1,
			Data: xdr.LedgerEntryData{
				Type:  xdr.LedgerEntryTypeOffer,
				Offer: &offer,
			},
		},
		Post: &updatedEntry,
	})
	s.Assert().NoError(err)

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:  xdr.LedgerEntryTypeOffer,
				Offer: &anotherOffer,
			},
		},
	})
	s.Assert().NoError(err)

	s.mockQ.On("UpsertOffers", s.ctx, mock.Anything).Run(func(args mock.Arguments) {
		// To fix order issue due to using ChangeCompactor
		offers := args.Get(1).([]history.Offer)
		s.Assert().ElementsMatch(
			offers,
			[]history.Offer{
				{
					SellerID:           "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
					OfferID:            2,
					Pricen:             int32(1),
					Priced:             int32(6),
					Price:              float64(1) / float64(6),
					LastModifiedLedger: uint32(lastModifiedLedgerSeq),
				},
				{
					SellerID:           "GDMUVYVYPYZYBDXNJWKFT3X2GCZCICTL3GSVP6AWBGB4ZZG7ZRDA746P",
					OfferID:            3,
					Pricen:             int32(2),
					Priced:             int32(3),
					Price:              float64(2) / float64(3),
					LastModifiedLedger: uint32(lastModifiedLedgerSeq),
				},
			},
		)
	}).Return(nil).Once()
	s.mockQ.On("CompactOffers", s.ctx, s.sequence-100).Return(int64(0), nil).Once()
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *OffersProcessorTestSuiteLedger) TestRemoveOffer() {
	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeOffer,
				Offer: &xdr.OfferEntry{
					SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					OfferId:  xdr.Int64(3),
					Price:    xdr.Price{3, 1},
				},
			},
		},
		Post: nil,
	})
	s.Assert().NoError(err)

	s.mockQ.On("RemoveOffers", s.ctx, []int64{3}, s.sequence).Return(int64(1), nil).Once()
	s.mockQ.On("CompactOffers", s.ctx, s.sequence-100).Return(int64(0), nil).Once()
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *OffersProcessorTestSuiteLedger) TestProcessUpgradeChange() {
	// add offer
	offer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		OfferId:  xdr.Int64(2),
		Price:    xdr.Price{1, 2},
	}
	lastModifiedLedgerSeq := xdr.Uint32(1234)

	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:  xdr.LedgerEntryTypeOffer,
				Offer: &offer,
			},
		},
	})
	s.Assert().NoError(err)

	updatedOffer := xdr.OfferEntry{
		SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		OfferId:  xdr.Int64(2),
		Price:    xdr.Price{1, 6},
	}

	updatedEntry := xdr.LedgerEntry{
		LastModifiedLedgerSeq: lastModifiedLedgerSeq,
		Data: xdr.LedgerEntryData{
			Type:  xdr.LedgerEntryTypeOffer,
			Offer: &updatedOffer,
		},
	}

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: lastModifiedLedgerSeq,
			Data: xdr.LedgerEntryData{
				Type:  xdr.LedgerEntryTypeOffer,
				Offer: &offer,
			},
		},
		Post: &updatedEntry,
	})
	s.Assert().NoError(err)

	// We use LedgerEntryChangesCache so all changes are squashed
	s.mockQ.On("UpsertOffers", s.ctx, []history.Offer{
		{
			SellerID:           "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
			OfferID:            2,
			Pricen:             int32(1),
			Priced:             int32(6),
			Price:              float64(1) / float64(6),
			LastModifiedLedger: uint32(lastModifiedLedgerSeq),
		},
	}).Return(nil).Once()

	s.mockQ.On("CompactOffers", s.ctx, s.sequence-100).Return(int64(0), nil).Once()
	s.Assert().NoError(s.processor.Commit(s.ctx))
}

func (s *OffersProcessorTestSuiteLedger) TestRemoveMultipleOffers() {
	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeOffer,
				Offer: &xdr.OfferEntry{
					SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					OfferId:  xdr.Int64(3),
					Price:    xdr.Price{3, 1},
				},
			},
		},
		Post: nil,
	})
	s.Assert().NoError(err)

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeOffer,
				Offer: &xdr.OfferEntry{
					SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					OfferId:  xdr.Int64(4),
					Price:    xdr.Price{3, 1},
				},
			},
		},
		Post: nil,
	})
	s.Assert().NoError(err)

	s.mockQ.On("CompactOffers", s.ctx, s.sequence-100).Return(int64(0), nil).Once()
	s.mockQ.On("RemoveOffers", s.ctx, mock.Anything, s.sequence).Run(func(args mock.Arguments) {
		// To fix order issue due to using ChangeCompactor
		ids := args.Get(1).([]int64)
		s.Assert().ElementsMatch(ids, []int64{3, 4})
	}).Return(int64(2), nil).Once()

	err = s.processor.Commit(s.ctx)
	s.Assert().NoError(err)
}

func (s *OffersProcessorTestSuiteLedger) TestRemoveMultipleOffersRowsAffectedCheck() {
	err := s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeOffer,
				Offer: &xdr.OfferEntry{
					SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					OfferId:  xdr.Int64(3),
					Price:    xdr.Price{3, 1},
				},
			},
		},
		Post: nil,
	})
	s.Assert().NoError(err)

	err = s.processor.ProcessChange(s.ctx, ingest.Change{
		Type: xdr.LedgerEntryTypeOffer,
		Pre: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeOffer,
				Offer: &xdr.OfferEntry{
					SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					OfferId:  xdr.Int64(4),
					Price:    xdr.Price{3, 1},
				},
			},
		},
		Post: nil,
	})
	s.Assert().NoError(err)

	s.mockQ.On("RemoveOffers", s.ctx, mock.Anything, s.sequence).Run(func(args mock.Arguments) {
		// To fix order issue due to using ChangeCompactor
		ids := args.Get(1).([]int64)
		s.Assert().ElementsMatch(ids, []int64{3, 4})
	}).Return(int64(0), nil).Once()

	err = s.processor.Commit(s.ctx)
	s.Assert().IsType(ingest.StateError{}, errors.Cause(err))
	s.Assert().EqualError(err, "error flushing cache: 0 rows affected when removing 2 offers")
}
