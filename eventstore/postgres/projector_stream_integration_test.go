// +build integration

package postgres_test

import (
	"context"
	"database/sql"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/hellofresh/goengine/aggregate"
	"github.com/hellofresh/goengine/eventstore/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type (
	streamProjectorTestSuite struct {
		projectorSuite
	}
)

func TestStreamProjectorSuite(t *testing.T) {
	suite.Run(t, new(streamProjectorTestSuite))
}

func (s *streamProjectorTestSuite) SetupTest() {
	s.projectorSuite.SetupTest()

	ctx := context.Background()
	queries := postgres.StreamProjectorCreateSchema("projections", s.eventStream, s.eventStoreTable)
	for _, query := range queries {
		_, err := s.DB().ExecContext(ctx, query)
		s.Require().NoError(err, "failed to create projection tables etc.")
	}
}

func (s *streamProjectorTestSuite) TearDownTest() {
	s.eventStore = nil
	s.eventStream = ""
	s.payloadTransformer = nil

	s.PostgresSuite.TearDownTest()
}

func (s *streamProjectorTestSuite) TestRun() {
	var wg sync.WaitGroup
	defer func() {
		if s.waitTimeout(&wg, 5*time.Second) {
			s.T().Fatal("projection.Run in go routines failed to return")
		}
	}()

	s.Require().NoError(
		s.payloadTransformer.RegisterPayload("account_debited", func() interface{} {
			return AccountDeposited{}
		}),
	)
	s.Require().NoError(
		s.payloadTransformer.RegisterPayload("account_credited", func() interface{} {
			return AccountCredited{}
		}),
	)

	projectorCtx, projectorCancel := context.WithCancel(context.Background())
	defer projectorCancel()

	projector, err := postgres.NewStreamProjector(
		s.PostgresDSN,
		s.eventStore,
		s.payloadTransformer,
		&DepositedProjection{},
		"projections",
		s.Logger,
	)
	s.Require().NoError(err, "failed to create projector")

	// Run the projector in the background
	wg.Add(1)
	go func() {
		if err := projector.Run(projectorCtx, true); err != nil {
			assert.NoError(s.T(), err, "projector.Run returned an error")
		}
		wg.Done()
	}()

	// Be evil and start run the projection again to ensure mutex is used and the context is respected
	wg.Add(1)
	go func() {
		if err := projector.Run(projectorCtx, true); err != nil {
			assert.NoError(s.T(), err, "projector.Run returned an error")
		}
		wg.Done()
	}()

	// Let the go routines start
	runtime.Gosched()

	// Add events to the event stream
	aggregateIds := []aggregate.ID{
		aggregate.GenerateID(),
	}
	s.appendEvents(map[aggregate.ID][]interface{}{
		aggregateIds[0]: {
			AccountDeposited{Amount: 100},
			AccountCredited{Amount: 50},
			AccountDeposited{Amount: 10},
			AccountDeposited{Amount: 5},
			AccountDeposited{Amount: 100},
			AccountDeposited{Amount: 1},
		},
	})
	s.expectProjectionState("deposited_report", 6, `{"Total": 5, "TotalAmount": 216}`)

	// Add events to the event stream
	s.appendEvents(map[aggregate.ID][]interface{}{
		aggregateIds[0]: {
			AccountDeposited{Amount: 100},
			AccountDeposited{Amount: 1},
		},
	})

	s.expectProjectionState("deposited_report", 8, `{"Total": 7, "TotalAmount": 317}`)

	projectorCancel()

	s.Run("projection should not rerun events", func() {
		projector, err := postgres.NewStreamProjector(
			s.PostgresDSN,
			s.eventStore,
			s.payloadTransformer,
			&DepositedProjection{},
			"projections",
			s.Logger,
		)
		s.Require().NoError(err, "failed to create projector")

		err = projector.Run(context.Background(), false)
		s.Require().NoError(err, "failed to run projector")

		s.expectProjectionState("deposited_report", 8, `{"Total": 7, "TotalAmount": 317}`)
	})
}

func (s *streamProjectorTestSuite) TestRun_Once() {
	s.Require().NoError(
		s.payloadTransformer.RegisterPayload("account_debited", func() interface{} {
			return AccountDeposited{}
		}),
	)
	s.Require().NoError(
		s.payloadTransformer.RegisterPayload("account_credited", func() interface{} {
			return AccountCredited{}
		}),
	)

	aggregateIds := []aggregate.ID{
		aggregate.GenerateID(),
	}
	// Add events to the event stream
	s.appendEvents(map[aggregate.ID][]interface{}{
		aggregateIds[0]: {
			AccountDeposited{Amount: 100},
			AccountCredited{Amount: 50},
			AccountDeposited{Amount: 10},
			AccountDeposited{Amount: 5},
			AccountDeposited{Amount: 100},
			AccountDeposited{Amount: 1},
		},
	})

	projector, err := postgres.NewStreamProjector(
		s.PostgresDSN,
		s.eventStore,
		s.payloadTransformer,
		&DepositedProjection{},
		"projections",
		s.Logger,
	)
	s.Require().NoError(err, "failed to create projector")

	s.Run("Run projections", func() {
		ctx := context.Background()

		err := projector.Run(ctx, false)
		s.Require().NoError(err)

		s.expectProjectionState("deposited_report", 6, `{"Total": 5, "TotalAmount": 216}`)

		s.Run("Run projection again", func() {
			// Append more events
			s.appendEvents(map[aggregate.ID][]interface{}{
				aggregateIds[0]: {
					AccountDeposited{Amount: 100},
					AccountDeposited{Amount: 1},
				},
			})

			err := projector.Run(ctx, false)
			s.Require().NoError(err)

			s.expectProjectionState("deposited_report", 8, `{"Total": 7, "TotalAmount": 317}`)
		})
	})
}

func (s *streamProjectorTestSuite) TestDelete() {
	var projectionExists bool

	projection := &DepositedProjection{}
	projector, err := postgres.NewStreamProjector(
		s.PostgresDSN,
		s.eventStore,
		s.payloadTransformer,
		projection,
		"projections",
		s.Logger,
	)
	s.Require().NoError(err, "failed to create projector")

	// Run the projection to ensure it exists
	err = projector.Run(context.Background(), false)
	s.Require().NoError(err, "failed to run projector")

	row := s.DB().QueryRow(`SELECT EXISTS(SELECT 1 FROM projections WHERE name = $1)`, projection.Name())
	s.Require().NoError(row.Scan(&projectionExists))
	s.Require().True(projectionExists, "projector.Run failed to create projection entry")

	// Remove projection
	err = projector.Delete(context.Background())
	s.Require().NoError(err)

	row = s.DB().QueryRow(`SELECT EXISTS(SELECT 1 FROM projections WHERE name = $1)`, projection.Name())
	s.Require().NoError(row.Scan(&projectionExists))
	s.Require().False(projectionExists)
}

func (s *streamProjectorTestSuite) expectProjectionState(name string, expectedPosition int64, expectedState string) {
	stmt, err := s.DB().Prepare(`SELECT position, state FROM projections WHERE name = $1`)
	s.Require().NoError(err)

	var (
		position int64
		state    string
	)

	for i := 0; i < 20; i++ {
		res := stmt.QueryRow(name)
		if err := res.Scan(&position, &state); err != nil {
			if err == sql.ErrNoRows {
				continue
			}

			s.Require().NoError(err)
			return
		}

		if position >= expectedPosition {
			s.Equal(expectedPosition, position)
			s.JSONEq(expectedState, state)
			return
		}

		// The expected state was not found to wait for a bit to allow the projector go routine/process to catch up
		time.Sleep(50 * time.Millisecond)
	}

	s.Require().Equal(expectedPosition, position, "failed to fetch expected projection state")
}
