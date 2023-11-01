package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/errors"
)

type processorsRunDurations map[string]time.Duration

func (d processorsRunDurations) AddRunDuration(name string, startTime time.Time) {
	d[name] += time.Since(startTime)
}

type groupChangeProcessors struct {
	processors []horizonChangeProcessor
	processorsRunDurations
}

func newGroupChangeProcessors(processors []horizonChangeProcessor) *groupChangeProcessors {
	return &groupChangeProcessors{
		processors:             processors,
		processorsRunDurations: make(map[string]time.Duration),
	}
}

func (g groupChangeProcessors) ProcessChange(ctx context.Context, change ingest.Change) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.ProcessChange(ctx, change); err != nil {
			return errors.Wrapf(err, "error in %T.ProcessChange", p)
		}
		g.AddRunDuration(fmt.Sprintf("%T", p), startTime)
	}
	return nil
}

func (g groupChangeProcessors) Commit(ctx context.Context) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.Commit(ctx); err != nil {
			return errors.Wrapf(err, "error in %T.Commit", p)
		}
		g.AddRunDuration(fmt.Sprintf("%T", p), startTime)
	}
	return nil
}

type groupTransactionProcessors struct {
	processors []horizonTransactionProcessor
	processorsRunDurations
}

func newGroupTransactionProcessors(processors []horizonTransactionProcessor) *groupTransactionProcessors {
	return &groupTransactionProcessors{
		processors:             processors,
		processorsRunDurations: make(map[string]time.Duration),
	}
}

func (g groupTransactionProcessors) ProcessTransaction(ctx context.Context, tx ingest.LedgerTransaction) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.ProcessTransaction(ctx, tx); err != nil {
			return errors.Wrapf(err, "error in %T.ProcessTransaction", p)
		}
		g.AddRunDuration(fmt.Sprintf("%T", p), startTime)
	}
	return nil
}

func (g groupTransactionProcessors) Commit(ctx context.Context) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.Commit(ctx); err != nil {
			return errors.Wrapf(err, "error in %T.Commit", p)
		}
		g.AddRunDuration(fmt.Sprintf("%T", p), startTime)
	}
	return nil
}

type groupTransactionFilterers struct {
	filterers []processors.LedgerTransactionFilterer
	processorsRunDurations
	droppedTransactions int64
}

func newGroupTransactionFilterers(filterers []processors.LedgerTransactionFilterer) *groupTransactionFilterers {
	return &groupTransactionFilterers{
		filterers:              filterers,
		processorsRunDurations: make(map[string]time.Duration),
	}
}

func (g *groupTransactionFilterers) FilterTransaction(ctx context.Context, tx ingest.LedgerTransaction) (bool, error) {
	for _, f := range g.filterers {
		startTime := time.Now()
		include, err := f.FilterTransaction(ctx, tx)
		if err != nil {
			return false, errors.Wrapf(err, "error in %T.FilterTransaction", f)
		}
		g.AddRunDuration(fmt.Sprintf("%T", f), startTime)
		if !include {
			// filter out, we can return early
			g.droppedTransactions++
			return false, nil
		}
	}
	return true, nil
}
